import "prosemirror-view/style/prosemirror.css";
import "prosemirror-gapcursor/style/gapcursor.css";
import { toggleMark } from "prosemirror-commands";
import { dropCursor } from "prosemirror-dropcursor";
import { gapCursor } from "prosemirror-gapcursor";
import { history } from "prosemirror-history";
import { type Command, EditorState } from "prosemirror-state";
import { EditorView } from "prosemirror-view";
import { forwardRef, type ReactNode, useEffect, useImperativeHandle, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Icon, type IconName } from "../ui/Icon";
import { useAutosize } from "../ui/useAutosize";
import { type CellHost, cellPlugins, cellView } from "./cells";
import { copyMarkdown, pasteMarkdown } from "./clipboard";
import { diagrams } from "./diagrams";
import { notesPlugin } from "./notes";
import { type RefHost, refsPlugin } from "./refs";
import {
  headingLevel,
  inside,
  insertRule,
  keys,
  linkAt,
  listActive,
  markActive,
  rules,
  setLink,
  toggleCode,
  toggleHeading,
  toggleList,
  toggleQuote,
} from "./commands";
import { type Markdown, parseMarkdown, saveMarkdown } from "./markdown";
import { schema } from "./schema";
import { highlighting, nodeViews, placeholder, type Quote, quoteMarks } from "./views";
import { Button } from "../ui/Button";
import { ToggleButton } from "../ui/ToggleButton";
import { Input } from "../ui/Input";
import { TextArea } from "../ui/TextArea";

// MarkdownEditor edits a markdown document as it reads: headings as
// headings, lists as lists, a task's box to tick. Markdown typed is
// formatting (# heading, - list, **bold**), the toolbar does the same, and
// Markdown shows the text itself, to edit as text.
//
// What it hands back is markdown that changes only what was edited
// (markdown.ts): a brief saved after one word changed differs from the
// one before by that word.

// Draft is where an edit stands: whether it's changed anything, whether
// there's anything in it, and its markdown (worked out when asked).
export interface Draft {
  dirty: boolean;
  empty: boolean;
  text: () => string;
}

export interface EditorProps {
  initial: string;
  // What it's editing, for screen readers: "The brief".
  label: string;
  placeholder?: string;
  autoFocus?: boolean;
  // Smaller, for a card (a proposal's brief).
  compact?: boolean;
  // Start in Markdown (the text itself), as for settling a clash.
  startIn?: "rich" | "source";
  // Words threads are on, highlighted as the reader highlights them.
  quotes?: Quote[];
  onChange?: (draft: Draft) => void;
  onSubmit?: () => void;
  onCancel?: () => void;
  // A page's: lines that start with >> become cells, which it fills
  // (cells.ts). onView hands it the editor while there is one (not in
  // Markdown), to write into.
  cells?: CellHost;
  onView?: (view: EditorView | undefined) => void;
  // A page's: what its passes found is drawn on the words it's about
  // (notes.ts), and this says when one's clicked.
  onNote?: (key: string) => void;
  // A page's: what its words name (a file, a mission) is drawn as what it
  // is, with a card on hover (refs.ts).
  refs?: RefHost;
  // A page with one toolbar for several documents (a mission's brief and
  // its plan): where this one's toolbar is drawn (toolbarIn, an element of
  // the page's) while it's the one being edited (toolbarOn), and onFocus
  // says when it's gone to. Without toolbarIn, its toolbar is over it.
  toolbarIn?: HTMLElement | null;
  toolbarOn?: boolean;
  onFocus?: () => void;
}

// placed is an editor's toolbar where it goes: over it, or in the page's
// one bar while it's the document being edited.
function placed(bar: ReactNode, props: Pick<EditorProps, "toolbarIn" | "toolbarOn">): ReactNode {
  if (props.toolbarIn === undefined) return bar;
  return props.toolbarIn && props.toolbarOn ? createPortal(bar, props.toolbarIn) : null;
}

const { nodes: N, marks: M } = schema;
const mac = /Mac|iPhone|iPad/.test(navigator.platform);
const mod = mac ? "⌘" : "Ctrl+";
const alt = mac ? "⌥" : "Alt+";
const shift = mac ? "⇧" : "Shift+";

export default function MarkdownEditor(props: EditorProps) {
  const [mode, setMode] = useState<"rich" | "source">(props.startIn ?? "rich");
  // The markdown the rich editor starts from: the initial text, or what
  // was written in Markdown mode.
  const [from, setFrom] = useState(props.initial);
  const [source, setSource] = useState(props.initial);
  const rich = useRef<RichHandle>(null);

  const toSource = () => {
    const text = rich.current?.markdown() ?? from;
    setSource(text);
    setMode("source");
  };
  const toRich = () => {
    setFrom(source);
    setMode("rich");
  };

  return (
    <div className={`doc-editor${props.compact ? " compact" : ""}`}>
      {mode === "rich" ? (
        <Rich key={from} ref={rich} {...props} markdown={from} onSource={toSource} />
      ) : (
        <Source {...props} text={source} setText={setSource} onRich={toRich} />
      )}
    </div>
  );
}

interface RichHandle {
  markdown(): string;
}

type RichProps = EditorProps & { markdown: string; onSource: () => void };

// Rich is the document drawn as it reads, edited in place.
const Rich = forwardRef<RichHandle, RichProps>(function Rich(props, ref) {
  const host = useRef<HTMLDivElement>(null);
  const [view, setView] = useState<EditorView>();
  const [, setTick] = useState(0);
  const [link, setLinkEdit] = useState<LinkEdit>();
  // The latest props, for the editor's callbacks (it's made once).
  const p = useRef(props);
  p.current = props;
  const md = useRef<Markdown>(undefined);

  useImperativeHandle(ref, () => ({ markdown: () => (view && md.current ? saveMarkdown(view.state.doc, md.current).text : props.markdown) }), [view, props.markdown]);

  useEffect(() => {
    const parsed = parseMarkdown(props.markdown);
    md.current = parsed;
    const report = (doc = parsed.doc) => {
      let text: string | undefined;
      p.current.onChange?.({
        dirty: !doc.eq(parsed.doc) || parsed.source !== p.current.initial,
        empty: !doc.textContent.trim(),
        text: () => (text ??= saveMarkdown(doc, parsed).text),
      });
    };
    const view: EditorView = new EditorView(host.current!, {
      state: EditorState.create({
        doc: parsed.doc,
        plugins: [
          history(),
          ...(props.cells ? cellPlugins(() => p.current.cells) : []),
          ...(props.onNote ? [notesPlugin((key) => p.current.onNote?.(key))] : []),
          ...(props.refs ? [refsPlugin(() => p.current.refs)] : []),
          rules,
          ...keys({ submit: () => p.current.onSubmit?.(), cancel: p.current.onCancel ? () => p.current.onCancel?.() : undefined, link: () => openLinkFor(view, setLinkEdit) }),
          dropCursor({ color: "var(--accent)" }),
          gapCursor(),
          highlighting,
          diagrams,
          placeholder(props.placeholder ?? ""),
          quoteMarks(props.quotes ?? []),
        ],
      }),
      nodeViews: props.cells ? { ...nodeViews, cell: cellView(() => p.current.cells) } : nodeViews,
      attributes: { class: "markdown pm", role: "textbox", "aria-multiline": "true", "aria-label": props.label, spellcheck: "true" },
      handleDOMEvents: {
        focus: () => {
          p.current.onFocus?.();
          return false;
        },
      },
      clipboardTextParser: (text, $context) => pasteMarkdown(text, $context),
      clipboardTextSerializer: (slice) => copyMarkdown(slice),
      handlePaste: (v, event) => {
        const text = event.clipboardData?.getData("text/plain") ?? "";
        const { from, to, empty, $from } = v.state.selection;
        // An address pasted over words links them.
        if (!empty && /^(https?:\/\/|mailto:)\S+$/.test(text.trim()) && !$from.parent.type.spec.code) {
          return setLink(from, to, text.trim())(v.state, v.dispatch);
        }
        // From a code editor, the text's markdown, not its coloured HTML.
        if (event.clipboardData?.types.includes("vscode-editor-data") && text) {
          v.dispatch(v.state.tr.replaceSelection(pasteMarkdown(text, $from)).scrollIntoView());
          return true;
        }
        return false;
      },
      dispatchTransaction(tr) {
        view.updateState(view.state.apply(tr));
        setTick((t) => t + 1);
        if (tr.docChanged) report(view.state.doc);
      },
    });
    setView(view);
    report();
    if (props.autoFocus) view.focus();
    p.current.onView?.(view);
    return () => {
      p.current.onView?.(undefined);
      view.destroy();
    };
    // Made once per document: a new one comes with a new key.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <>
      {placed(<Toolbar view={view} onLink={() => view && openLinkFor(view, setLinkEdit)} onSource={props.onSource} />, props)}
      {/* Drawn as the reader draws a document (.prose), so nothing moves
          when you start editing. */}
      <div className="doc-body prose">
        <div ref={host} />
        {view && link && <LinkBox view={view} edit={link} close={() => setLinkEdit(undefined)} />}
      </div>
    </>
  );
});

interface LinkEdit {
  from: number;
  to: number;
  href: string;
  top: number;
  left: number;
}

// openLinkFor asks for the address of the link the cursor's in, or of the
// words selected.
function openLinkFor(view: EditorView, set: (l: LinkEdit) => void) {
  const at = linkAt(view.state);
  const { from, to } = at ?? view.state.selection;
  const c = view.coordsAtPos(from);
  const box = view.dom.getBoundingClientRect();
  set({ from, to, href: at?.href ?? "", top: c.bottom - box.top + 6, left: Math.max(0, c.left - box.left) });
}

// LinkBox asks for a link's address, under the words it's on.
function LinkBox({ view, edit, close }: { view: EditorView; edit: LinkEdit; close: () => void }) {
  const [href, setHref] = useState(edit.href);
  const done = (to?: string) => {
    if (to !== undefined) setLink(edit.from, edit.to, to.trim())(view.state, view.dispatch);
    close();
    view.focus();
  };
  return (
    <div className="doc-link" style={{ top: edit.top, left: edit.left }}>
      <Input
        autoFocus
        value={href}
        placeholder="https://…"
        aria-label="Link address"
        onChange={(e) => setHref(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            done(href);
          }
          if (e.key === "Escape") {
            e.preventDefault();
            done();
          }
        }}
        onBlur={() => close()}
      />
      <Button variant="primary" size="small" onMouseDown={(e) => e.preventDefault()} onClick={() => done(href)}>
        {edit.href ? "Change" : "Link"}
      </Button>
      {edit.href && (
        <Button variant="ghost" size="small" onMouseDown={(e) => e.preventDefault()} onClick={() => done("")}>
          Remove
        </Button>
      )}
    </div>
  );
}

// Toolbar is what the keys and the typed markdown do, as buttons, each
// lit when the selection's in it.
function Toolbar({ view, onLink, onSource }: { view?: EditorView; onLink: () => void; onSource: () => void }) {
  const state = view?.state;
  const run = (cmd: Command) => () => {
    if (!view) return;
    cmd(view.state, view.dispatch, view);
    view.focus();
  };
  const b = (key: string, icon: IconName | string, title: string, on: boolean, cmd: () => void) => (
    <ToggleButton
      variant="ghost" square
      className="doc-tool"
      key={key}
      pressed={on}
      title={title}
      aria-label={title}
      disabled={!view}
      // Keep the editor's selection: the button acts on it.
      onMouseDown={(e) => e.preventDefault()}
      onChange={cmd}
    >
      {icon.length <= 2 ? <span className="h">{icon}</span> : <Icon name={icon as IconName} />}
    </ToggleButton>
  );
  const level = state ? headingLevel(state) : 0;
  const parts: ReactNode[] = [
    ...[1, 2, 3].map((n) => b(`h${n}`, `H${n}`, `Heading ${n} (${mod}${alt}${n})`, level === n, run(toggleHeading(n)))),
    <span key="s1" className="sep" />,
    b("bold", "bold", `Bold (${mod}B)`, !!state && markActive(state, M.strong), run(toggleMark(M.strong))),
    b("italic", "italic", `Italic (${mod}I)`, !!state && markActive(state, M.em), run(toggleMark(M.em))),
    b("strike", "strike", `Strikethrough (${mod}${shift}X)`, !!state && markActive(state, M.strike), run(toggleMark(M.strike))),
    b("code", "code", `Code (${mod}E)`, !!state && markActive(state, M.code), run(toggleMark(M.code))),
    b("link", "link", `Link (${mod}K)`, !!state && markActive(state, M.link), onLink),
    <span key="s2" className="sep" />,
    b("bullet", "list", `Bullets (${mod}${shift}8)`, !!state && listActive(state, "bullet"), run(toggleList("bullet"))),
    b("ordered", "list-ordered", `Numbers (${mod}${shift}7)`, !!state && listActive(state, "ordered"), run(toggleList("ordered"))),
    b("task", "list-checks", `Tasks (${mod}${shift}9)`, !!state && listActive(state, "task"), run(toggleList("task"))),
    b("quote", "quote", `Quote (${mod}${shift}.)`, !!state && inside(state, N.blockquote), run(toggleQuote)),
    b("codeblock", "code-block", "Code block (```)", !!state && state.selection.$from.parent.type === N.code_block, run(toggleCode)),
    b("rule", "minus", "Rule (---)", false, run(insertRule)),
  ];
  return (
    <div className="doc-toolbar" role="toolbar" aria-label="Formatting">
      {parts}
      <span className="spacer" />
      <ToggleButton variant="ghost" size="small" pressed={false} onChange={onSource} title="Edit the markdown itself">
        <Icon name="hash" size={12} /> Markdown
      </ToggleButton>
    </div>
  );
}

type SourceProps = EditorProps & { text: string; setText: (t: string) => void; onRich: () => void };

// Source is the document's markdown, to edit as text.
function Source({ text, setText, onRich, initial, label, placeholder, onChange, onSubmit, onCancel, toolbarIn, toolbarOn, onFocus }: SourceProps) {
  const ref = useRef<HTMLTextAreaElement>(null);
  useAutosize(ref, text, 4000);
  const change = useRef(onChange);
  change.current = onChange;
  useEffect(() => {
    change.current?.({ dirty: text !== initial, empty: !text.trim(), text: () => text });
  }, [text, initial]);
  useEffect(() => ref.current?.focus(), []);
  return (
    <>
      {placed(
        <div className="doc-toolbar" role="toolbar" aria-label="Formatting">
          <span className="dim">Markdown, as it's saved</span>
          <span className="spacer" />
          <ToggleButton variant="ghost" size="small" pressed onChange={onRich} title="Back to the document as it reads">
            <Icon name="hash" size={12} /> Markdown
          </ToggleButton>
        </div>,
        { toolbarIn, toolbarOn },
      )}
      <TextArea
        ref={ref}
        className="doc-source"
        value={text}
        placeholder={placeholder}
        aria-label={`${label}, in markdown`}
        onFocus={onFocus}
        onChange={(e) => setText(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && (e.metaKey || e.ctrlKey) && onSubmit) {
            e.preventDefault();
            onSubmit();
          }
          if (e.key === "Escape" && onCancel) {
            e.preventDefault();
            onCancel();
          }
        }}
      />
    </>
  );
}
