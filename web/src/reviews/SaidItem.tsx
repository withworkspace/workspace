import type { ReactNode } from "react";
import { Markdown } from "../markdown/LazyMarkdown";
import css from "./SaidItem.module.css";

// SaidItem is one thing said on a pull request on GitHub: its head (who,
// what they did, when), then what they said. A bot's is folded: it's
// rarely what you came to read.
export function SaidItem({ head, body, bot }: { head: ReactNode; body?: string; bot?: boolean }) {
  const said = body && (
    <div className={`prose ${css.body}`}>
      <Markdown text={body} />
    </div>
  );
  if (bot) {
    return (
      <details className={css.said}>
        <summary>{head}</summary>
        {said}
      </details>
    );
  }
  return (
    <article className={css.said}>
      <header>{head}</header>
      {said}
    </article>
  );
}
