import { history } from "prosemirror-history";
import { EditorState, type Plugin, Selection } from "prosemirror-state";
import { EditorView } from "prosemirror-view";
import { useEffect, useRef } from "react";
import { cx } from "../ui/cx";
import type { Handoff, RichTextAreaProps } from "../ui/RichTextArea";
import css from "../ui/RichTextArea.module.css";
import { useSendKey } from "../ui/sendKey";
import { pasteMarkdown } from "./clipboard";
import { setLink } from "./commands";
import { composeKeys, composeRules, fit, placeholder, tidy } from "./compose";
import { type Markdown, parseMarkdown, saveMarkdown } from "./markdown";
import editorCss from "./MarkdownEditor.module.css";
import { refsPlugin } from "./refs";

// RichText is RichTextArea (ui/RichTextArea.tsx) once its editor's here:
// ProseMirror on the document editor's schema, offering what its props
// say, its words handed back as markdown as they change.
//
// What it offers is read once, when it's drawn: a box doesn't change what
// it is. What's written, the key that sends and the callbacks are read as
// they are.
export default function RichText(props: RichTextAreaProps & { handoff: Handoff }) {
  const host = useRef<HTMLDivElement>(null);
  const view = useRef<EditorView>(undefined);
  const yours = useSendKey();
  const p = useRef({ ...props, sendKey: props.send ?? yours });
  p.current = { ...props, sendKey: props.send ?? yours };
  // What value was last, as read or said: a value other than it is new.
  const last = useRef(props.value);
  // The markdown the words were last read from: what's written keeps its
  // style (its bullets, its emphasis) where it isn't changed.
  const base = useRef<Markdown>(undefined);

  const stateOf = (value: string, plugins: readonly Plugin[]) => {
    const parsed = parseMarkdown(value);
    base.current = parsed;
    return EditorState.create({ doc: parsed.doc, plugins, selection: Selection.atEnd(parsed.doc) });
  };

  useEffect(() => {
    const f = { marks: props.marks, lists: props.lists, quote: props.quote, code: props.code };
    const plugins = [
      history(),
      composeRules(f),
      ...composeKeys(f, {
        send: () => p.current.sendKey,
        onSend: (how) => p.current.onSend?.(how),
      }),
      ...(props.refs ? [refsPlugin(() => p.current.refs)] : []),
      placeholder(() => p.current.placeholder ?? "", css.empty),
    ];
    const v: EditorView = new EditorView(host.current!, {
      state: stateOf(props.value, plugins),
      attributes: () => ({
        class: cx(css.doc, props.refs && editorCss.refs),
        role: "textbox",
        "aria-multiline": "true",
        "aria-label": p.current["aria-label"],
        ...(p.current.busy ? { "aria-busy": "true" } : {}),
        spellcheck: "true",
      }),
      clipboardTextParser: (text, $context) => pasteMarkdown(text, $context),
      transformPasted: (slice) => fit(slice, f),
      handlePaste: (view, event) => {
        if (!f.marks) return false;
        // An address pasted over words links them.
        const text = event.clipboardData?.getData("text/plain")?.trim() ?? "";
        const { from, to, empty, $from } = view.state.selection;
        if (empty || !/^(https?:\/\/|mailto:)\S+$/.test(text) || $from.parent.type.spec.code) return false;
        return setLink(from, to, text)(view.state, view.dispatch);
      },
      // Escape is the page's unless the box hears it: then it's handed the
      // press, to keep from the page if it will.
      handleKeyDown: (_view, e) => {
        if (e.key !== "Escape" || e.isComposing || !p.current.onEscape) return false;
        e.preventDefault();
        p.current.onEscape(e);
        return true;
      },
      handleDOMEvents: {
        focus: () => {
          p.current.handoff.focused = true;
          p.current.onFocus?.();
          return false;
        },
        blur: () => {
          p.current.handoff.focused = false;
          p.current.onBlur?.();
          return false;
        },
      },
      dispatchTransaction(tr) {
        v.updateState(v.state.apply(tr));
        if (!tr.docChanged) return;
        const text = tidy(saveMarkdown(v.state.doc, base.current).text);
        if (text === last.current) return;
        last.current = text;
        p.current.onChange(text);
      },
    });
    view.current = v;
    setField(p.current.field, v.dom);
    if (props.autoFocus || props.handoff.focused) v.focus();
    return () => {
      setField(p.current.field, null);
      view.current = undefined;
      v.destroy();
    };
    // Made once: what it offers is what it was drawn with.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // A value from the page (emptied once sent, a draft come back): read anew.
  useEffect(() => {
    const v = view.current;
    if (!v || props.value === last.current) return;
    last.current = props.value;
    v.updateState(stateOf(props.value, v.state.plugins));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [props.value]);

  // What it says of itself to a screen reader, as it changes.
  useEffect(() => {
    view.current?.setProps({});
  }, [props["aria-label"], props.busy, props.placeholder]);

  return (
    <div
      className={cx(css.area, props.className)}
      style={{ "--rows": props.rows ?? 1, "--tallest": `${props.tallest ?? 320}px` } as React.CSSProperties}
      data-rich=""
      ref={host}
    />
  );
}

function setField(field: RichTextAreaProps["field"], el: HTMLElement | null) {
  if (typeof field === "function") field(el);
  else if (field) (field as React.MutableRefObject<HTMLElement | null>).current = el;
}
