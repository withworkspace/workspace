import css from "./Spinner.module.css";
import { cx } from "./cx";
import { Icon } from "./Icon";

// Spinner is something going on now: an arc that turns, the size of the
// icon it stands in for. Beside words it says what ("Looking…"); alone it
// takes a title.
export function Spinner({ size = 14, className, title }: { size?: number; className?: string; title?: string }) {
  return <Icon name="spinner" size={size} className={cx(css.spinner, className)} title={title} />;
}
