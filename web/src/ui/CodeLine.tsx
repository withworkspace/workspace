import { memo, type ReactNode } from "react";
import css from "./CodeLine.module.css";
import type { CodeMark } from "./codeLayout";
import { CodeText } from "./CodeText";
import { cx } from "./cx";
import { Icon } from "./Icon";

// CodeLine is one line of CodeLines: its number (pressed, it picks the
// line), its mark where it's changed, the twist that folds what's under
// it, and its code. Folded, it says how many lines are away, and that
// opens them again. Under it is whatever the page puts after the line
// (after: a comment on it), which stays in view however far the code's
// scrolled sideways.
//
// Its code is a CodeText, which says where the line is for what follows
// a name in it.
export const CodeLine = memo(function CodeLine({
  n,
  html,
  path,
  rev,
  side,
  mark,
  picked,
  folds,
  fold,
  shut,
  onFold,
  after,
}: {
  n: number;
  html: string;
  path: string;
  rev?: string;
  side: "old" | "new";
  mark?: CodeMark;
  picked: boolean;
  folds: boolean; // the file has folds: every line keeps their column
  fold?: number; // the last line that folds away under this one
  shut: boolean;
  onFold: (at: number) => void;
  after?: ReactNode;
}) {
  const marks: Record<CodeMark, string | undefined> = { added: css.added, modified: css.modified, removedAfter: css.removedAfter };
  const away = fold ? fold - n : 0;
  const code = <CodeText className={css.code} html={html} rev={rev} path={path} line={n} side={side} />;
  return (
    <>
      <div data-line={n} className={cx(css.line, folds && css.folds, mark && marks[mark], picked && css.picked)}>
        <button type="button" data-ui="" data-number={n} className={css.number} tabIndex={-1} aria-label={`Line ${n}`}>
          {n}
        </button>
        <span className={css.mark} aria-hidden />
        {folds &&
          (fold ? (
            <button
              type="button"
              data-ui=""
              className={cx(css.twist, shut && css.shut)}
              onClick={() => onFold(n)}
              tabIndex={-1}
              aria-expanded={!shut}
              aria-label={`${shut ? "Unfold" : "Fold"} ${away === 1 ? `line ${fold}` : `lines ${n + 1} to ${fold}`}`}
            >
              <Icon name={shut ? "chevron-right" : "chevron-down"} size={12} />
            </button>
          ) : (
            <span aria-hidden />
          ))}
        {shut ? (
          <span className={css.folded}>
            {code}
            <button type="button" data-ui="" className={css.more} onClick={() => onFold(n)} tabIndex={-1} title="Unfold">
              ⋯ {away} {away === 1 ? "line" : "lines"}
            </button>
          </span>
        ) : (
          code
        )}
      </div>
      {after && (
        <div className={css.after} data-after={n}>
          {after}
        </div>
      )}
    </>
  );
});
