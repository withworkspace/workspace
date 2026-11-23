import { useEffect, useRef, useState } from "react";
import { useKeys } from "../keys";
import { Button } from "./Button";
import { useAutosize } from "./useAutosize";
import css from "./ReplyBox.module.css";
import { TextArea } from "./TextArea";

// ReplyBox is you answering what a card says, in the card: it takes the
// keys when it's drawn, ⌘⏎ sends, esc doesn't. While it's there the
// page's keys are off.
export function ReplyBox({ onSend, onCancel, placeholder = "Reply…", send = "Reply" }: { onSend: (body: string) => void; onCancel: () => void; placeholder?: string; send?: string }) {
  const [text, setText] = useState("");
  const area = useRef<HTMLTextAreaElement>(null);
  useAutosize(area, text, 240);
  useKeys({}, { modal: true });
  useEffect(() => {
    area.current?.focus();
  }, []);
  return (
    <div className={css.box} onClick={(e) => e.stopPropagation()}>
      <TextArea
        ref={area}
        className={css.field}
        value={text}
        rows={2}
        placeholder={placeholder}
        aria-label={send}
        onChange={(e) => setText(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && (e.metaKey || e.ctrlKey) && text.trim()) {
            e.preventDefault();
            onSend(text);
          } else if (e.key === "Escape") {
            e.preventDefault();
            onCancel();
          }
        }}
      />
      <div className={css.actions}>
        <Button variant="ghost" size="small" onClick={onCancel} title="Cancel (esc)">
          Cancel
        </Button>
        <Button variant="primary" size="small" disabled={!text.trim()} onClick={() => onSend(text)} title="Send it (⌘⏎)">
          {send}
        </Button>
      </div>
    </div>
  );
}
