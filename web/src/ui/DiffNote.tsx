import { cx } from "./cx";
import css from "./DiffNote.module.css";
import type { DiffNoteRow } from "./diffLayout";

// DiffNote is what a diff says in place of lines: it's a binary file,
// it's empty, or it's too large to show until it's asked for. That last
// is pressed for the lines (onShow); the rest are only words.
export function DiffNote({ row, cursor, onShow }: { row: DiffNoteRow; cursor: boolean; onShow?: (row: DiffNoteRow) => void }) {
  if (!row.large) {
    return (
      <div data-row={row.key} className={cx(css.note, cursor && css.cursor)}>
        {row.text}
      </div>
    );
  }
  return (
    <button type="button" data-ui="" data-row={row.key} tabIndex={-1} className={cx(css.note, css.pressed, cursor && css.cursor)} onClick={() => onShow?.(row)}>
      {row.text}
    </button>
  );
}
