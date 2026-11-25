import { type ReactNode, useLayoutEffect, useRef, useState } from "react";
import { type DiffAt } from "../ui/DiffLine";
import { DiffLines } from "../ui/DiffLines";
import type { DiffCell, DiffPick } from "../ui/diffLayout";
import type { Row } from "./model";
import css from "./ReviewLines.module.css";

// ReviewLines is a file's rows in a review (ui/DiffLines draws them): its
// lines, the gaps between its hunks, and among them what the page draws
// itself (draw: a comment, what a pass found, the editor). under is what
// floats under one of its rows, over the lines below it and at the code's
// end (the picked lines' bar): it's over them and not among them, so
// nothing moves under the pointer while lines are picked.
export function ReviewLines({
  label,
  rows,
  split,
  html,
  at,
  cursor,
  picked,
  onPick,
  onRow,
  onGap,
  onNote,
  draw,
  under,
}: {
  label: string;
  rows: Row[];
  split: boolean;
  html: (cell: DiffCell, side: "old" | "new") => string;
  at?: DiffAt;
  cursor?: string;
  picked?: DiffPick;
  onPick?: (pick: DiffPick) => void;
  onRow?: (key: string, how: { shift: boolean; number: boolean }) => void;
  onGap?: (row: Row) => void;
  onNote?: (row: Row) => void;
  draw?: (row: Row) => ReactNode;
  under?: { row: string; what: ReactNode };
}) {
  const box = useRef<HTMLDivElement>(null);
  // Where the row it's under ends, down the box.
  const [top, setTop] = useState<number>();
  const underRow = under?.row;
  useLayoutEffect(() => {
    const el = underRow === undefined ? null : box.current?.querySelector<HTMLElement>(`[data-row="${CSS.escape(underRow)}"]`);
    setTop(el && box.current ? el.getBoundingClientRect().bottom - box.current.getBoundingClientRect().top : undefined);
  }, [underRow, rows, split]);
  return (
    <div className={css.box} ref={box}>
      <DiffLines aria-label={label} className={css.lines} rows={rows} split={split} html={html} at={at} cursor={cursor} picked={picked} onPick={onPick} onRow={onRow} onGap={onGap} onNote={onNote} draw={draw} />
      {under && top !== undefined && (
        <div className={css.under} style={{ top }}>
          {under.what}
        </div>
      )}
    </div>
  );
}
