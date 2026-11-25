import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./PullRequestSubmitLayer.module.css";

// pullRequestLeftAloneClass is "Left alone", said after a layer's
// verdicts while it has none.
export const pullRequestLeftAloneClass = css.alone;

// PullRequestSubmitLayer is one pull request's review in the dialog that
// posts it: its place in the stack (n), title, ref and comments, how it's
// going (state), then its verdict, summary and what will be posted. The
// one the keys are on is focus; one posted is quiet. Alone, it's not a
// card.
export function PullRequestSubmitLayer({ n, title, at, said, state, focus, posted, onFocus, children }: { n?: number; title: string; at: ReactNode; said: string; state?: ReactNode; focus: boolean; posted: boolean; onFocus: () => void; children?: ReactNode }) {
  return (
    <section className={cx(css.layer, focus && css.focus, posted && css.posted)} onClick={onFocus}>
      <header>
        {n !== undefined && <span className={css.n}>{n}</span>}
        <span className={css.title}>{title}</span>
        <span className="mono dim">{at}</span>
        <span className="dim">{said}</span>
        <span className={css.spacer} />
        {state}
      </header>
      {children}
    </section>
  );
}
