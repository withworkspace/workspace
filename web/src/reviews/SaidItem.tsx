import type { ReactNode } from "react";
import { Markdown } from "../markdown/LazyMarkdown";
import css from "./SaidItem.module.css";
import { Prose } from "../ui/Prose";

// SaidItem is one thing said on a pull request on GitHub: its head (who,
// what they did, when), then what they said. A bot's is folded: it's
// rarely what you came to read. Its markdown is as GitHub draws a
// comment: a line break is one (breaks).
export function SaidItem({ head, body, bot }: { head: ReactNode; body?: string; bot?: boolean }) {
  const said = body && (
    <Prose className={css.body}>
      <Markdown text={body} breaks />
    </Prose>
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
