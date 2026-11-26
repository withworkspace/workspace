import { useEffect, useRef, useState } from "react";
import { cx } from "../ui/cx";
import { Input } from "../ui/Input";
import css from "./SettingsField.module.css";
import { textClass } from "../ui/Text";

type SettingsFieldProps = {
  // What it's at, and what it goes back to.
  value: string;
  label: string;
  placeholder?: string;
  // short is for a few characters (a number of days); mono for what's
  // typed as an agent names it; stretch for one in a table's cell.
  short?: boolean;
  mono?: boolean;
  stretch?: boolean;
  // focused takes the keys as it comes (in place of a list it was picked
  // from).
  focused?: boolean;
  // What's typed, once it's kept: Enter, or leaving the field, if it
  // changed and isn't empty.
  onCommit: (text: string) => void;
  // Told once it's left, whatever came of it.
  onDone?: () => void;
};

// SettingsField is a setting that's typed: kept on Enter or leaving it,
// if it changed; Escape leaves it as it was.
export function SettingsField({ value, label, placeholder, short, mono, stretch, focused, onCommit, onDone }: SettingsFieldProps) {
  const [text, setText] = useState(value);
  useEffect(() => setText(value), [value]);
  const cancelled = useRef(false);
  const field = useRef<HTMLInputElement>(null);
  // What it's come in place of holds the keys until it's closed: they're
  // taken once it has.
  useEffect(() => {
    if (!focused) return;
    const frame = requestAnimationFrame(() => field.current?.focus());
    return () => cancelAnimationFrame(frame);
  }, [focused]);
  const commit = () => {
    const t = text.trim();
    if (cancelled.current || t === "" || t === value) setText(value);
    else onCommit(t);
    cancelled.current = false;
    onDone?.();
  };
  return (
    <Input
      ref={field}
      type="text"
      className={cx(css.field, mono && textClass("mono"), short && css.short, stretch && css.stretch)}
      value={text}
      placeholder={placeholder}
      spellCheck={false}
      aria-label={label}
      onChange={(e) => setText(e.target.value)}
      onBlur={commit}
      onKeyDown={(e) => {
        if (e.key === "Enter") e.currentTarget.blur();
        if (e.key === "Escape") {
          cancelled.current = true;
          e.currentTarget.blur();
        }
      }}
    />
  );
}
