import { useState } from "react";
import css from "./EditDiff.module.css";
import { Button } from "./Button";
import { cx } from "./cx";

// Edit is what an agent changed in a file: what it replaced, and with
// what. A file written whole has nothing replaced.
export interface Edit {
  path: string;
  old?: string;
  new: string;
  all?: boolean;
}

// EditDiff is an edit as a diff: the lines it replaced on red, over the
// lines it put there on green (a file written whole is all green). A long
// one shows its start (max lines), and the rest when asked.
export function EditDiff({ edit, max = 14 }: { edit: Edit; max?: number }) {
  const [all, setAll] = useState(false);
  const old = edit.old ? edit.old.split("\n") : [];
  const next = edit.new.split("\n");
  const total = old.length + next.length;
  const cut = (lines: string[], room: number) => (all ? lines : lines.slice(0, Math.max(room, 0)));
  const oldShown = cut(old, Math.ceil(max / 2));
  const newShown = cut(next, max - oldShown.length);
  return (
    <div className={css.diff}>
      {oldShown.map((l, i) => (
        <div key={`o${i}`} className={cx(css.line, css.was)}>
          <span className={css.sign}>−</span>
          {l || " "}
        </div>
      ))}
      {newShown.map((l, i) => (
        <div key={`n${i}`} className={cx(css.line, css.now)}>
          <span className={css.sign}>+</span>
          {l || " "}
        </div>
      ))}
      {!all && total > oldShown.length + newShown.length && (
        <Button variant="link" className={css.more} onClick={() => setAll(true)}>
          {total - oldShown.length - newShown.length} more lines
        </Button>
      )}
    </div>
  );
}
