import { cx } from "../ui/cx";
import css from "./DocumentsSpaceTag.module.css";
import { RepoDot } from "../ui/RepoDot";

// DocumentsSpaceTag is which space something's in, where there's more
// than one: its repo's colour and its name; at the line's end (end), in
// a row that has room.
export function DocumentsSpaceTag({ label, color, end }: { label: string; color: string; end?: boolean }) {
  return (
    <span className={cx(css.tag, end && css.end)}>
      <RepoDot color={color} className={css.dot} />
      {label}
    </span>
  );
}
