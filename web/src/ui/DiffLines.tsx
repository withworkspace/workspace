import { type CSSProperties, type ReactNode, useMemo, useRef } from "react";
import { cx } from "./cx";
import { DiffGap } from "./DiffGap";
import { type DiffAt, DiffLine } from "./DiffLine";
import css from "./DiffLines.module.css";
import { DiffNote } from "./DiffNote";
import { type DiffCell, type DiffPick, digitsOf, isGap, isLine, isNote, type Keyed, pickedRows, pickFrom, bandAt } from "./diffLayout";

// How many rows are in a run: what's laid out together, or not at all.
const RUN = 200;

// DiffLines is one file's diff to read: its rows, in one column (unified)
// or old and new side by side (split). It draws rows it's given and reads
// nothing. The rows are as review/model lays a file out (fileRows): a
// line, a gap of unchanged lines between hunks, a note in place of lines.
// Any other row among them is the page's to draw (draw: a comment on the
// lines above it, what a pass found, an editor), level with the code.
//
//   html     a cell's line, highlighted or escaped, on its side
//   at       where each side is read, for following names in its lines
//   picked   the lines picked, tinted; onPick is told when a number's
//            pressed (that line), shift-pressed (from the first picked to
//            it) or dragged along (the run). Side by side, the pick is of
//            the column its number is in
//   cursor   the row the page's keys are on; onRow is told when a row's
//            pressed, for the page to put them there, and how: with
//            shift held, and whether it was a number (which is a pick,
//            told to onPick already)
//   onGap    a gap's pressed: the page reads the file and opens it. With
//            every gap open it's the whole file, the change in place
//   onNote   a note that stands for lines (a large diff) is pressed
//
// A row of the page's own between two lines of one kind (a comment among
// added lines) carries their colour on behind it, in one column: the run
// reads as one change with something said in it. What it draws stands
// over the lines after it, so a card's shadow falls on them.
//
// Long lines wrap: a diff's two sides can't each scroll sideways. Rows
// far from what's in view aren't laid out, so a long diff opens as a
// short one does.
export function DiffLines<R extends Keyed>({
  rows,
  split = false,
  html,
  at,
  picked,
  onPick,
  cursor,
  onRow,
  onGap,
  onNote,
  draw,
  className,
  "aria-label": label,
}: {
  rows: R[];
  split?: boolean;
  html: (cell: DiffCell, side: "old" | "new") => string;
  at?: DiffAt;
  picked?: DiffPick;
  onPick?: (pick: DiffPick) => void;
  cursor?: string;
  onRow?: (key: string, how: { shift: boolean; number: boolean }) => void;
  onGap?: (row: R) => void;
  onNote?: (row: R) => void;
  draw?: (row: R) => ReactNode;
  className?: string;
  "aria-label": string;
}) {
  const box = useRef<HTMLDivElement>(null);
  const latest = useRef({ rows, picked, onPick, onRow });
  latest.current = { rows, picked, onPick, onRow };
  const span = useMemo(() => pickedRows(rows, picked), [rows, picked]);
  const digits = useMemo(() => digitsOf(rows), [rows]);

  // Picking: a number pressed is its line, with shift it's from the first
  // line picked to it, and dragged along the numbers it's the run.
  const dragging = useRef<{ anchor: string; side?: "old" | "new" }>(undefined);
  const rowAt = (e: React.PointerEvent) => {
    const el = document.elementFromPoint(e.clientX, e.clientY)?.closest<HTMLElement>("[data-row]");
    return el && box.current?.contains(el) ? el.dataset.row : undefined;
  };
  const down = (e: React.PointerEvent) => {
    const { rows: all, picked: was, onPick: pick } = latest.current;
    const number = (e.target as HTMLElement).closest<HTMLElement>("[data-pick]");
    if (!pick || !number || e.button !== 0) return;
    const key = number.dataset.pick!;
    const side = number.dataset.side as "old" | "new" | undefined;
    const anchor = e.shiftKey && was ? was.from : key;
    const next = pickFrom(all, anchor, key, side);
    if (!next) return;
    dragging.current = { anchor, side };
    pick(next);
    e.preventDefault();
  };
  const move = (e: React.PointerEvent) => {
    const drag = dragging.current;
    if (!drag) return;
    const { rows: all, picked: was, onPick: pick } = latest.current;
    const key = rowAt(e);
    const next = key === undefined ? undefined : pickFrom(all, drag.anchor, key, drag.side);
    if (next && pick && (next.from !== was?.from || next.to !== was?.to)) pick(next);
  };
  const up = () => {
    dragging.current = undefined;
  };
  // A press on a row puts the page's keys there; a number's press is a
  // pick, and what's pressed in a row of the page's own is its own.
  const press = (e: React.MouseEvent) => {
    const target = e.target as HTMLElement;
    const el = target.closest<HTMLElement>("[data-row]");
    if (el?.dataset.row !== undefined) latest.current.onRow?.(el.dataset.row, { shift: e.shiftKey, number: !!target.closest("[data-pick]") });
  };

  const runs: ReactNode[] = [];
  for (let start = 0; start < rows.length; start += RUN) {
    const run = rows.slice(start, start + RUN).map((row, i) => {
      const n = start + i;
      const here = row.key === cursor;
      if (isLine(row)) {
        const tint = span && n >= span.from && n <= span.to ? ((split && picked?.side) || "all") : undefined;
        return <DiffLine key={row.key} row={row} split={split} left={row.left && html(row.left, "old")} right={row.right && html(row.right, "new")} at={at} cursor={here} picked={tint} />;
      }
      if (isGap(row)) return <DiffGap key={row.key} row={row} cursor={here} onShow={() => onGap?.(row)} />;
      if (isNote(row)) return <DiffNote key={row.key} row={row} cursor={here} onShow={() => onNote?.(row)} />;
      const drawn = draw?.(row);
      const band = drawn && !split ? bandAt(rows, n) : undefined;
      return (
        drawn && (
          <div key={row.key} data-row={row.key} className={cx(css.other, split && css.beside, band === "a" && css.added, band === "d" && css.removed)}>
            {drawn}
          </div>
        )
      );
    });
    runs.push(
      <div key={start} className={css.run}>
        {run}
      </div>,
    );
  }

  return (
    <div
      ref={box}
      role="group"
      aria-label={label}
      className={cx(css.lines, className)}
      style={{ "--diff-digits": digits } as CSSProperties}
      onPointerDown={down}
      onPointerMove={move}
      onPointerUp={up}
      onPointerLeave={up}
      onClick={press}
    >
      {runs}
    </div>
  );
}
