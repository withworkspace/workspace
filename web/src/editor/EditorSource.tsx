import { useEffect, useRef } from "react";
import { cx } from "../ui/cx";
import { TextArea } from "../ui/TextArea";
import { useAutosize } from "../ui/useAutosize";
import { EditorToolbar } from "./EditorToolbar";
import css from "./EditorSource.module.css";
import type { EditorProps } from "./MarkdownEditor";
import { placed } from "./placed";
import { Text } from "../ui/Text";

type EditorSourceProps = EditorProps & { text: string; setText: (t: string) => void; onRich: () => void };

// EditorSource is the document's markdown, to edit as text.
export function EditorSource({ text, setText, onRich, initial, label, placeholder, compact, onChange, onSubmit, onCancel, toolbarIn, toolbarOn, onFocus }: EditorSourceProps) {
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
        <EditorToolbar compact={compact && toolbarIn === undefined} placed={toolbarIn !== undefined} source onSource={onRich}>
          <Text dim>Markdown, as it's saved</Text>
        </EditorToolbar>,
        { toolbarIn, toolbarOn },
      )}
      <TextArea
        ref={ref}
        className={cx(css.source, compact && css.compact)}
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
