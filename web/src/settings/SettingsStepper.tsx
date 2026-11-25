import { useEffect, useRef, useState } from "react";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Input } from "../ui/Input";
import { toast } from "../ui/toast";
import css from "./SettingsStepper.module.css";

type SettingsStepperProps = {
  // The number it's at, and as its field says it.
  value: number;
  text: string;
  label: string;
  onChange: (n: number) => void;
};

// SettingsStepper is a whole number: typed, or stepped by one.
export function SettingsStepper({ value, text: was, label, onChange }: SettingsStepperProps) {
  const [text, setText] = useState(was);
  useEffect(() => setText(was), [was]);
  const cancelled = useRef(false);
  const commit = () => {
    const t = text.trim();
    const cancel = cancelled.current;
    cancelled.current = false;
    if (cancel || t === "" || t === was) return setText(was);
    // What isn't a number would be sent as nothing, which clears the key.
    if (!/^\d+$/.test(t)) {
      setText(was);
      return toast(`${label} is a whole number.`, { tone: "danger" });
    }
    onChange(Number(t));
  };
  return (
    <span className={css.stepper}>
      <Button type="button" disabled={value <= 1} title="One fewer" aria-label={`${label}: one fewer`} onClick={() => onChange(value - 1)}>
        <Icon name="minus" size={12} />
      </Button>
      <Input
        type="text"
        inputMode="numeric"
        value={text}
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
      <Button type="button" title="One more" aria-label={`${label}: one more`} onClick={() => onChange(value + 1)}>
        <Icon name="plus" size={12} />
      </Button>
    </span>
  );
}
