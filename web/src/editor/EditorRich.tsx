import { dropCursor } from "prosemirror-dropcursor";
import { gapCursor } from "prosemirror-gapcursor";
import { history } from "prosemirror-history";
import { EditorState } from "prosemirror-state";
import { EditorView } from "prosemirror-view";
import { forwardRef, useEffect, useImperativeHandle, useRef, useState } from "react";
import { markdownClass } from "../markdown/LazyMarkdown";
import { cx } from "../ui/cx";
import { cellPlugins, cellView } from "./cells";
import { copyMarkdown, pasteMarkdown } from "./clipboard";
import { keys, linkAt, rules, setLink } from "./commands";
import { diagrams } from "./diagrams";
import { EditorLinkBox } from "./EditorLinkBox";
import { EditorToolbar } from "./EditorToolbar";
import { EditorTools } from "./EditorTools";
import { type Markdown, parseMarkdown, saveMarkdown } from "./markdown";
import type { EditorProps } from "./MarkdownEditor";
import css from "./MarkdownEditor.module.css";
import { notesPlugin } from "./notes";
import { placed } from "./placed";
import { refsPlugin } from "./refs";
import { highlighting, nodeViews, placeholder, quoteMarks } from "./views";

export interface RichHandle {
  markdown(): string;
}

type RichProps = EditorProps & { markdown: string; onSource: () => void };

// EditorRich is the document drawn as it reads, edited in place.
export const EditorRich = forwardRef<RichHandle, RichProps>(function EditorRich(props, ref) {
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
      // markdown and pm: the names other places still know the document
      // by (the refs' rules, the brief's, the app's keys).
      attributes: { class: cx("markdown pm", markdownClass, css.pm, props.compact && css.compact), role: "textbox", "aria-multiline": "true", "aria-label": props.label, spellcheck: "true" },
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

  const openLink = () => view && openLinkFor(view, setLinkEdit);
  const linked = (to?: string) => {
    if (view && link && to !== undefined) setLink(link.from, link.to, to.trim())(view.state, view.dispatch);
    setLinkEdit(undefined);
    view?.focus();
  };

  return (
    <>
      {placed(
        <EditorToolbar compact={props.compact && props.toolbarIn === undefined} placed={props.toolbarIn !== undefined} onSource={props.onSource}>
          <EditorTools view={view} onLink={openLink} />
        </EditorToolbar>,
        props,
      )}
      {/* Drawn as the reader draws a document (.prose), so nothing moves
          when you start editing. */}
      <div className={cx(css.body, "prose")}>
        <div ref={host} />
        {view && link && <EditorLinkBox href={link.href} top={link.top} left={link.left} onDone={linked} onClose={() => setLinkEdit(undefined)} />}
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
