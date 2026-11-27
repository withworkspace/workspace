import { reviewKeys } from "./keys";
import type { Command } from "../keymap/commands";
import { useSaid } from "../keymap/keymap";
import { titleWithKeys } from "../ui/keyHint";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import { TLink } from "../ui/TLink";
import css from "./ReviewSideBack.module.css";

// ReviewSideBack is a review's way back (esc, or the key you've set), as the first row of the
// list down its side: where it's from (Reviews, or the mission). Its rule
// is level with the bar's over the pane, so the two read as one line.
export function ReviewSideBack({ to, label, keys }: { to: string; label: string; keys?: Command }) {
  const said = useSaid(keys ?? reviewKeys.back);
  return (
    <div className={css.back}>
      <TLink to={to} className={css.link} title={titleWithKeys(`Back to ${label}`, said)}>
        <Icon name="arrow-left" size={14} className={css.icon} />
        <span className={css.words}>{label}</span>
      </TLink>
      {said && <Keys k={said} className={css.keys} />}
    </div>
  );
}
