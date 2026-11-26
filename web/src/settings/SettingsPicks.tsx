import { useRef, useState } from "react";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Input } from "../ui/Input";
import css from "./SettingsPicks.module.css";
import { textClass } from "../ui/Text";

type SettingsPicksProps = {
  picks: string[];
  label: string;
  // What's typed in an empty list's field, to show what one looks like.
  example: string;
  // Those typed that the list hasn't got, to add.
  onAdd: (more: string[]) => void;
  // One taken off: by its ×, or Backspace from the empty field.
  onDrop: (i: number) => void;
};

// SettingsPicks is a list (of harness:model), each its own: taken off
// with its ×, or Backspace from the empty field; one more added by typing
// it, then Enter, a comma, or leaving the field.
export function SettingsPicks({ picks, label, example, onAdd, onDrop }: SettingsPicksProps) {
  const [text, setText] = useState("");
  const field = useRef<HTMLInputElement>(null);
  const add = () => {
    const more = text
      .split(/[\s,]+/)
      .filter(Boolean)
      .filter((p, i, all) => !picks.includes(p) && all.indexOf(p) === i);
    setText("");
    if (more.length > 0) onAdd(more);
  };
  return (
    // A press on the box anywhere is a press on its field.
    <div
      className={css.picks}
      onMouseDown={(e) => {
        if (e.target !== e.currentTarget) return;
        e.preventDefault();
        field.current?.focus();
      }}
    >
      {picks.map((p, i) => (
        <span key={p} className={css.pick}>
          {p}
          <Button type="button" title={`Remove ${p}`} aria-label={`Remove ${p}`} onClick={() => onDrop(i)}>
            <Icon name="x" size={10} />
          </Button>
        </span>
      ))}
      <Input
        ref={field}
        type="text"
        className={textClass("mono")}
        value={text}
        placeholder={picks.length > 0 ? "add another" : example}
        spellCheck={false}
        aria-label={`${label}: add one`}
        onChange={(e) => setText(e.target.value)}
        onBlur={add}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === ",") {
            e.preventDefault();
            add();
          }
          if (e.key === "Backspace" && text === "" && picks.length > 0) onDrop(picks.length - 1);
          if (e.key === "Escape") setText("");
        }}
      />
    </div>
  );
}
