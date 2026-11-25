import type { CSSProperties, MouseEvent, ReactNode, Ref } from "react";
import { cx } from "../../ui/cx";
import { Icon } from "../../ui/Icon";
import css from "./FilesCode.module.css";

// FilesCode is a file's text as the explorer shows it, as an editor would:
// no card around it, its lines numbered in a gutter as wide as its last
// number (digits), each a FilesCodeLine. What's drawn over the lines (a
// name's card, the picker of its uses) comes before them.
export function FilesCode({ digits, ref, children }: { digits: number; ref?: Ref<HTMLDivElement>; children: ReactNode }) {
  return (
    <div className={css.code} ref={ref} style={{ "--ln-digits": digits } as CSSProperties}>
      {children}
    </div>
  );
}

export type LineMark = "added" | "modified" | "deleted-after";

type FilesCodeLineProps = {
  n: number;
  // Its text, highlighted (HTML), as the highlighter gives it.
  html: string;
  // How it's changed since the branch point: added, changed, or lines
  // removed after it.
  mark?: LineMark;
  // In the lines the address picks (#L12-L14).
  picked?: boolean;
  // Where a declaration, a scope or a run of lines further in starts: it
  // folds away to its last line (to), and is folded (shut).
  fold?: { to: number; shut: boolean };
  // Where its names are read: the commit, the file.
  rev?: string;
  path: string;
  onPick: (e: MouseEvent) => void;
  onFold: () => void;
};

const marks: Record<LineMark, string> = { added: css.added, modified: css.modified, "deleted-after": css.deletedAfter };

// FilesCodeLine is one line of a FilesCode: its number, which links to it
// (shift-click for a range), its change mark, the fold beside the first
// line of what folds, and its text, or, folded, what's folded away after.
export function FilesCodeLine({ n, html, mark, picked, fold, rev, path, onPick, onFold }: FilesCodeLineProps) {
  const shut = !!fold?.shut;
  const code = <code data-rev={rev} data-path={path} data-line={n} data-side="new" dangerouslySetInnerHTML={{ __html: html || " " }} />;
  return (
    <div data-line={n} className={cx(css.line, mark && marks[mark], picked && css.picked, shut && css.shut)}>
      <button className={css.ln} onClick={onPick} tabIndex={-1} aria-label={`Line ${n}`}>
        {n}
      </button>
      <span className={css.mark} aria-hidden />
      {fold && (
        <button className={css.fold} onClick={onFold} tabIndex={-1} aria-expanded={!shut} aria-label={`${shut ? "Unfold" : "Fold"} ${fold.to === n + 1 ? `line ${fold.to}` : `lines ${n + 1} to ${fold.to}`}`}>
          <Icon name={shut ? "chevron-right" : "chevron-down"} size={12} />
        </button>
      )}
      {shut && fold ? (
        <span className={css.folded}>
          {code}
          <button className={css.more} onClick={onFold} tabIndex={-1} title="Unfold">
            ⋯ {fold.to - n} {fold.to - n === 1 ? "line" : "lines"}
          </button>
        </span>
      ) : (
        code
      )}
    </div>
  );
}
