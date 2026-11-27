import { useEffect, useRef, useState } from "react";
import { type NoteDraft, type NoteKind, noteKinds, type ReviewNote } from "../api";
import { useKeys } from "../keys";
import { Button } from "../ui/Button";
import { hintOnClick } from "../ui/keyHint";
import { Keys } from "../ui/Keys";
import { SendKeys } from "../ui/SendKeys";
import { RichTextArea } from "../ui/RichTextArea";
import { sendKeys, useSendKey } from "../ui/sendKey";
import { capital } from "../ui/strings";
import { linesWord } from "./findings";
import { joinSuggestion, splitSuggestion, suggestionBlock } from "./words";
import css from "./ReviewNoteEditor.module.css";
import { Text } from "../ui/Text";

// ReviewNoteEditor writes a comment on a review's lines (target), or
// rewrites a draft's (note): its kind (tab goes round them), what it
// says, and a change it suggests, which starts from the lines it's on
// (lines). It saves with the key you've chosen to send with (ui/sendKey.ts),
// and esc leaves it. While it's open, the page's keys are its own. What's
// written is markdown, drawn as it reads (RichTextArea), as GitHub will
// show it: a suggested change is its ```suggestion block of code.
export function ReviewNoteEditor({
  target,
  note,
  lines,
  onSave,
  onCancel,
}: {
  target: { side: "old" | "new"; line: number; endLine: number };
  note?: ReviewNote;
  lines: string[];
  onSave: (n: NoteDraft) => void;
  onCancel: () => void;
}) {
  const [kind, setKind] = useState<NoteKind>(note?.kind || "suggestion");
  const [text, setText] = useState(() => (note ? joinSuggestion(note.body, note.suggestion) : ""));
  const area = useRef<HTMLElement>(null);
  const sendKey = useSendKey();
  useKeys({}, { modal: true });
  useEffect(() => {
    area.current?.focus();
    area.current?.scrollIntoView({ block: "nearest" });
  }, []);
  const save = () => {
    const { body, suggestion } = splitSuggestion(text);
    if (!body && suggestion == null) return;
    onSave({ kind, body, suggestion });
  };
  const suggest = () => {
    if (suggestionBlock.test(text)) return;
    setText((t) => joinSuggestion(t.trim(), lines.join("\n")));
    area.current?.focus();
  };
  return (
    <div className={css.editor}>
      <div className={css.head}>
        {noteKinds.map((k) => (
          <button key={k} className={`kind k-${k}${k === kind ? " on" : ""}`} onClick={hintOnClick("tab", () => setKind(k))} aria-pressed={k === kind}>
            {capital(k)}
          </button>
        ))}
        <Text dim>
          {target.side === "old" ? "removed " : ""}
          {linesWord(target)} · <Keys k="tab" /> kind
        </Text>
      </div>
      <RichTextArea
        marks
        lists
        quote
        code
        refs
        look="field"
        className={css.field}
        field={area}
        value={text}
        onChange={setText}
        rows={Math.min(12, Math.max(3, text.split("\n").length + 1))}
        tallest={480}
        placeholder="What should change, or what do you want to know?"
        aria-label={note ? "The comment" : "A comment"}
        send={sendKey}
        onSend={save}
        onEscape={onCancel}
        onKeyDown={(e) => {
          if (e.key !== "Tab" || e.metaKey || e.ctrlKey || e.altKey) return false;
          const i = noteKinds.indexOf(kind);
          setKind(noteKinds[(i + (e.shiftKey ? noteKinds.length - 1 : 1)) % noteKinds.length]);
          return true;
        }}
      />
      <div className={css.actions}>
        <Button onClick={suggest} disabled={suggestionBlock.test(text)}>
          Suggest a change
        </Button>
        <span className={css.spacer} />
        <Button onClick={onCancel} keys="esc">
          Cancel <Keys k="esc" />
        </Button>
        <Button variant="primary" onClick={save} keys={sendKeys(sendKey)}>
          {note ? "Save" : "Comment"} <SendKeys />
        </Button>
      </div>
    </div>
  );
}
