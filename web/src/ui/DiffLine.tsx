import { memo } from "react";
import { CodeText } from "./CodeText";
import { cx } from "./cx";
import css from "./DiffLine.module.css";
import { type DiffCell, type DiffLineRow, unifiedKind } from "./diffLayout";

// DiffAt is where a diff's sides are read, for following names in its
// lines: the commit and the file's path, before the change and with it.
export interface DiffAt {
  old: { rev: string; path: string };
  new: { rev: string; path: string };
}

// DiffLine is a line of a diff. In one column (unified): the old side's
// number, the new side's, a sign, and the code, on green where it's added
// and red where it's removed. Side by side (split): each side's number
// and code, with a hatch where a side has no line. Its numbers are where
// lines are picked from (DiffLines hears the press: data-pick is the row,
// data-side the column), and a picked line is tinted: all of it, or side
// by side the column it was picked in. cursor is where the page's keys
// are.
export const DiffLine = memo(function DiffLine({
  row,
  split,
  left,
  right,
  at,
  cursor,
  picked,
}: {
  row: DiffLineRow;
  split: boolean;
  left?: string; // the old side's line, highlighted or escaped
  right?: string;
  at?: DiffAt;
  cursor: boolean;
  picked?: "all" | "old" | "new";
}) {
  const kinds: Record<DiffCell["kind"], string | undefined> = { a: css.added, d: css.removed, c: undefined };
  const number = (c: DiffCell | undefined, side: "old" | "new", tint: boolean, kind?: DiffCell["kind"]) =>
    c ? (
      <button type="button" data-ui="" data-pick={row.key} data-side={split ? side : undefined} tabIndex={-1} className={cx(css.number, kinds[kind ?? c.kind], tint && css.picked)} aria-label={`${side === "old" ? "Old line" : "Line"} ${c.num}`}>
        {c.num}
      </button>
    ) : (
      <span className={cx(css.number, split ? css.none : kind && kinds[kind], tint && css.picked)} aria-hidden />
    );
  const code = (c: DiffCell | undefined, html: string | undefined, side: "old" | "new", tint: boolean, kind?: DiffCell["kind"]) =>
    c ? (
      <CodeText className={cx(css.code, kinds[kind ?? c.kind], c.shown && css.shown, tint && css.picked)} html={html ?? ""} rev={at?.[side].rev} path={at?.[side].path} line={c.num} side={side} noeol={c.noeol} />
    ) : (
      <span className={cx(css.code, css.none)} aria-hidden />
    );

  if (split) {
    const old = picked === "all" || picked === "old";
    const now = picked === "all" || picked === "new";
    return (
      <div data-row={row.key} className={cx(css.line, css.split, cursor && css.cursor)}>
        {number(row.left, "old", old)}
        {code(row.left, left, "old", old)}
        {number(row.right, "new", now)}
        {code(row.right, right, "new", now)}
      </div>
    );
  }
  const kind = unifiedKind(row);
  const c = row.right ?? row.left!;
  const side = kind === "d" ? "old" : "new";
  const tint = !!picked;
  return (
    <div data-row={row.key} className={cx(css.line, cursor && css.cursor)}>
      {number(row.left, "old", tint, kind)}
      {number(row.right, "new", tint, kind)}
      <span className={cx(css.sign, kinds[kind], tint && css.picked)} aria-hidden>
        {kind === "a" ? "+" : kind === "d" ? "−" : ""}
      </span>
      {code(c, side === "old" ? left : right, side, tint, kind)}
    </div>
  );
});
