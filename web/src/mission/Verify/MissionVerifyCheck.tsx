import type { ReactNode } from "react";
import css from "./MissionVerifyCheck.module.css";

type MissionVerifyCheckProps = {
  // What it came to: a chip.
  verdict: ReactNode;
  // Which check it is, and who says so.
  meta: ReactNode;
  title: ReactNode;
  // What was said of it, and its points.
  said?: ReactNode;
  points?: ReactNode[];
  // What shows it: what the judge said, what ws printed, its files, where
  // it's done, how to see it yourself, what its crew suggest.
  children?: ReactNode;
  // What's yours of it, at the pane's foot: a MissionVerifyYours.
  yours?: ReactNode;
};

// MissionVerifyCheck is one check of the open bundle, in its pane: what
// it came to, what was said of it, and what shows it.
export function MissionVerifyCheck({ verdict, meta, title, said, points, children, yours }: MissionVerifyCheckProps) {
  return (
    <article className={css.check} aria-label="The check, and what shows it">
      <header>
        <div className={css.top}>
          {verdict}
          <span className={css.meta}>{meta}</span>
        </div>
        <h3>{title}</h3>
        {said && <p className={css.said}>{said}</p>}
        {points?.length ? (
          <ul className={css.points}>
            {points.map((pt, i) => (
              <li key={i}>{pt}</li>
            ))}
          </ul>
        ) : null}
      </header>
      {children}
      {yours}
    </article>
  );
}
