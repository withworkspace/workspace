import type { ReactNode } from "react";
import css from "./Change.module.css";
import { cx } from "./cx";

type Hunk = { was: string[]; now: string[] };

type ChangeProps = {
  // The lines as they read, and as they would. Or several such, each a
  // passage of its own (hunks), with a rule between.
  was?: string[];
  now?: string[];
  hunks?: Hunk[];
  // What it is, over the lines ("Suggested change").
  title?: ReactNode;
  // − and + before each line: code's, where a line's start matters.
  signs?: boolean;
  // The lines are words, which wrap; not code, which scrolls.
  wrap?: boolean;
  // What was there is struck through as well.
  struck?: boolean;
  // Said where there's nothing in their place ("(delete these lines)").
  gone?: ReactNode;
  // The lines alone, with no box: in a card that's one already.
  bare?: boolean;
  // The page's own class, to place it.
  className?: string;
};

// Change is what something would change: the lines as they read, on
// red, over the lines as they would, on green. A suggestion on a diff,
// an edit to a document, what a revision changed of a brief.
export function Change({ was = [], now = [], hunks, title, signs, wrap, struck, gone, bare, className }: ChangeProps) {
  const all = hunks ?? [{ was, now }];
  return (
    <div className={cx(css.change, wrap && css.wrap, bare && css.bare, title != null && css.titled, className)} data-change="">
      {title != null && <div className={css.title}>{title}</div>}
      {all.map((h, i) => (
        <div key={i} className={css.hunk}>
          {h.was.map((l, j) => (
            <div key={`w${j}`} className={cx(css.line, css.was, struck && css.struck)}>
              {signs ? `− ${l}` : l || " "}
            </div>
          ))}
          {h.now.length === 0 && gone != null ? (
            <div className={cx(css.line, css.gone)}>{gone}</div>
          ) : (
            h.now.map((l, j) => (
              <div key={`n${j}`} className={cx(css.line, css.now)}>
                {signs ? `+ ${l}` : l || " "}
              </div>
            ))
          )}
        </div>
      ))}
    </div>
  );
}
