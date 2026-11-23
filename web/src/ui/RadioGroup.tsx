import type { ReactNode } from "react";
import { Radio as AriaRadio, RadioGroup as AriaRadioGroup } from "react-aria-components";
import { cx } from "./cx";
import css from "./RadioGroup.module.css";

// RadioGroup is one of a few, each with words that say what choosing it
// means, down a column: whose a repo is, where a thing lives. (One of a
// few said in a word each, side by side, is a Segmented.) value is the
// one chosen, and onChange is called with another when it's picked.
// react-aria's radio group does the rest: it's one stop for Tab, the
// arrows move the choice, and a screen reader hears a group of radios,
// named by aria-label.
export function RadioGroup<T extends string>({
  value,
  onChange,
  disabled,
  className,
  "aria-label": label,
  children,
}: {
  value: T;
  onChange: (value: T) => void;
  disabled?: boolean;
  className?: string;
  "aria-label": string;
  children: ReactNode; // Radios
}) {
  return (
    <AriaRadioGroup className={cx(css.group, className)} value={value} onChange={(v) => onChange(v as T)} isDisabled={disabled} aria-label={label}>
      {children}
    </AriaRadioGroup>
  );
}

// Radio is one of them: its value, and its words, which can run to more
// than a line: the dot sits by the first.
export function Radio({ value, className, children }: { value: string; className?: string; children: ReactNode }) {
  return (
    <AriaRadio value={value} className={cx(css.radio, className)}>
      <span className={css.dot} aria-hidden="true" />
      {children}
    </AriaRadio>
  );
}
