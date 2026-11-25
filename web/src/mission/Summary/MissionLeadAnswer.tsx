import type { FormEvent, ReactNode } from "react";
import css from "./MissionLeadAnswer.module.css";

// MissionLeadAnswer is what a mission's card offers, under it: a
// MissionLeadAnswerForm where words are written (an answer, a reply to
// crew), MissionLeadButtons in a row, and a link on.
export function MissionLeadAnswer({ children }: { children: ReactNode }) {
  return <div className={css.answer}>{children}</div>;
}

// MissionLeadAnswerForm is a field and what sends it, in a line.
export function MissionLeadAnswerForm({ onSubmit, children }: { onSubmit: () => void; children: ReactNode }) {
  return (
    <form
      className={css.form}
      onSubmit={(e: FormEvent) => {
        e.preventDefault();
        onSubmit();
      }}
    >
      {children}
    </form>
  );
}

// MissionLeadButtons is what a card offers, in a row that wraps.
export function MissionLeadButtons({ children }: { children: ReactNode }) {
  return <div className={css.buttons}>{children}</div>;
}
