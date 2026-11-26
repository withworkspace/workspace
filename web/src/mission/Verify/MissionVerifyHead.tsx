import type { ReactNode } from "react";
import css from "./MissionVerifyHead.module.css";

type MissionVerifyHeadProps = {
  id: string;
  // What it's of: its step, or its capsule.
  title: ReactNode;
  // Who made it, of what, and when.
  meta: ReactNode;
  // Its capsule hasn't moved since: what it shows is of the capsule as it is.
  current?: boolean;
  // Under those: what you've said of it (MissionVerifyNotes), its
  // summary, its tally, and what's folded (MissionVerifyFolds).
  children?: ReactNode;
};

// MissionVerifyHead is the open bundle: who made it and of what, whether
// it's still of its capsule as it is, and what's said of it.
export function MissionVerifyHead({ id, title, meta, current, children }: MissionVerifyHeadProps) {
  return (
    <section className={css.head} aria-label="Bundle">
      <div className={css.top}>
        <h2>
          <span className="ref">{id}</span> {title}
        </h2>
        <span className={css.meta}>{meta}</span>
        {current && <span className={css.current}>Current: capsule unchanged since</span>}
      </div>
      {children}
    </section>
  );
}
