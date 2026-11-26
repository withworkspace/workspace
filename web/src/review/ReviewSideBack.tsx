import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import { TLink } from "../ui/TLink";
import css from "./ReviewSideBack.module.css";

// ReviewSideBack is a review's way back (esc), as the first row of the
// list down its side: where it's from (Reviews, or the mission). Its rule
// is level with the bar's over the pane, so the two read as one line.
export function ReviewSideBack({ to, label }: { to: string; label: string }) {
  return (
    <div className={css.back}>
      <TLink to={to} className={css.link} title={`Back to ${label} (esc)`}>
        <Icon name="arrow-left" size={14} className={css.icon} />
        <span className={css.words}>{label}</span>
      </TLink>
      <Keys k="esc" className={css.keys} />
    </div>
  );
}
