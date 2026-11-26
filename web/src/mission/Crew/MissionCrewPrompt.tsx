import type { FormEvent, ReactNode } from "react";
import { Link } from "react-router";
import { cx } from "../../ui/cx";
import { Icon } from "../../ui/Icon";
import { Button } from "../../ui/Button";
import { Input } from "../../ui/Input";
import { toneClass } from "../../ui/tone";
import css from "./MissionCrewPrompt.module.css";

type MissionCrewPromptProps = {
  // A permission (danger) or a question (attention).
  kind: "permission" | "question";
  // What it says, after the icon: who wants what.
  head: ReactNode;
  // As the mission's Summary shows it: no shadow.
  compact?: boolean;
  "aria-label": string;
  children: ReactNode;
};

// MissionCrewPrompt is what crew are stopped on: a permission (the step it
// wants to take, and allow or deny) or a question (its options, or your
// own words), as a card in the tone of it. In it are what it was given
// (MissionCrewPromptInput), the way to the file (MissionCrewPromptFile),
// the options (MissionCrewPromptOptions), your own answer
// (MissionCrewPromptAnswer), and the buttons (MissionCrewPromptActions).
export function MissionCrewPrompt({ kind, head, compact, "aria-label": label, children }: MissionCrewPromptProps) {
  return (
    <div className={cx(css.card, toneClass(kind === "permission" ? "danger" : "attention"), compact && css.compact)} role="group" aria-label={label}>
      <div className={css.head}>
        <Icon name={kind === "permission" ? "lock" : "question"} size={14} className={css.icon} />
        <span>{head}</span>
      </div>
      {children}
    </div>
  );
}

// MissionCrewPromptPath is the file it wants to change, by its folder and
// name, in the head.
export function MissionCrewPromptPath({ children }: { children: ReactNode }) {
  return <code className={css.path}>{children}</code>;
}

// MissionCrewPromptInput is what it was given to run, or what its screen
// says of what it asks.
export function MissionCrewPromptInput({ children }: { children: ReactNode }) {
  return <pre className={css.input}>{children}</pre>;
}

// MissionCrewPromptFile is the way to the file it asks leave to change.
export function MissionCrewPromptFile({ to }: { to: string }) {
  return (
    <Link to={to} className={css.file}>
      <Icon name="file" size={12} /> Current file
    </Link>
  );
}

// MissionCrewPromptOptions are a question's own answers.
export function MissionCrewPromptOptions({ children }: { children: ReactNode }) {
  return <div className={css.options}>{children}</div>;
}

// MissionCrewPromptAnswer is an answer in your own words.
export function MissionCrewPromptAnswer({ value, onChange, onSubmit, busy }: { value: string; onChange: (v: string) => void; onSubmit: () => void; busy: boolean }) {
  return (
    <form
      className={css.answer}
      onSubmit={(e: FormEvent) => {
        e.preventDefault();
        onSubmit();
      }}
    >
      <Input value={value} onChange={(e) => onChange(e.target.value)} placeholder="Or in your own words" aria-label="Your answer" disabled={busy} />
      <Button variant="primary" type="submit" disabled={busy || !value.trim()}>
        Answer
      </Button>
    </form>
  );
}

// MissionCrewPromptActions are what's pressed, and at their end (more) the
// way to answer in the terminal instead.
export function MissionCrewPromptActions({ children, more }: { children?: ReactNode; more?: ReactNode }) {
  return (
    <div className={css.actions}>
      {children}
      <span className={css.spacer} />
      {more}
    </div>
  );
}
