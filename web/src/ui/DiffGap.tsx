import { cx } from "./cx";
import css from "./DiffGap.module.css";
import type { DiffGapRow } from "./diffLayout";
import { Icon } from "./Icon";

// DiffGap is unchanged lines not shown, between a diff's hunks: how many,
// and what they're in. Pressed, it asks for them (onShow): the page reads
// the file and opens the gap. Past the last hunk it doesn't know how many
// there are until the file's read, and says only that there's more.
export function DiffGap({ row, cursor, onShow }: { row: DiffGapRow; cursor: boolean; onShow?: (row: DiffGapRow) => void }) {
  const words = row.count === undefined ? "Show more" : `Show ${row.count} unchanged ${row.count === 1 ? "line" : "lines"}`;
  return (
    <button type="button" data-ui="" data-row={row.key} tabIndex={-1} className={cx(css.gap, cursor && css.cursor)} onClick={() => onShow?.(row)}>
      <Icon name={row.count === undefined ? "chevron-down" : "more"} size={13} />
      <span>{words}</span>
      {row.section && <span className={css.section}>{row.section}</span>}
    </button>
  );
}
