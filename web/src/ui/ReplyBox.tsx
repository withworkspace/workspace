import { useState } from "react";
import { useCommands } from "../keymap/useCommands";
import { quietKeys } from "./partKeys";
import { Button } from "./Button";
import { RichTextArea } from "./RichTextArea";
import { sendKeys, useSendKey } from "./sendKey";
import css from "./ReplyBox.module.css";

// ReplyBox is you answering what a card says, in the card: it takes the
// keys when it's drawn, the key you've chosen sends (ui/sendKey.ts), esc
// doesn't. While it's there the page's keys are off. What's written is
// markdown, drawn as it reads (RichTextArea): marks, lists, a quote, code.
export function ReplyBox({ onSend, onCancel, placeholder = "Reply…", send = "Reply" }: { onSend: (body: string) => void; onCancel: () => void; placeholder?: string; send?: string }) {
  const [text, setText] = useState("");
  const key = useSendKey();
  useCommands(quietKeys, {}, { modal: true });
  return (
    <div className={css.box} onClick={(e) => e.stopPropagation()}>
      <RichTextArea
        marks
        lists
        quote
        code
        refs
        look="field"
        className={css.field}
        value={text}
        onChange={setText}
        rows={2}
        tallest={240}
        autoFocus
        placeholder={placeholder}
        aria-label={send}
        send={key}
        onSend={() => {
          if (text.trim()) onSend(text);
        }}
        onEscape={onCancel}
      />
      <div className={css.actions}>
        <Button variant="ghost" size="small" onClick={onCancel} title="Cancel" keys="esc">
          Cancel
        </Button>
        <Button variant="primary" size="small" disabled={!text.trim()} onClick={() => onSend(text)} title="Send" keys={sendKeys(key)}>
          {send}
        </Button>
      </div>
    </div>
  );
}
