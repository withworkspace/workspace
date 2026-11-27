import { useSaid } from "../keymap/keymap";
import { everywhereKeys } from "../shell/keys";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { hintOnClick } from "../ui/keyHint";
import { Keys } from "../ui/Keys";
import css from "./FindButton.module.css";

// FindButton is the sidebar's way to Find: what it's called, and its key
// (yours, where you've set one). In the rail it's its icon.
export function FindButton({
  onPress,
  className,
}: {
  onPress?: () => void;
  className?: string;
}) {
  const key = useSaid(everywhereKeys.find);
  return (
    <button
      type="button"
      data-ui=""
      aria-label="Find"
      className={cx(css.find, className)}
      onClick={hintOnClick(key, onPress)}
    >
      <Icon name="search" size={14} />
      <span className={css.words}>Find…</span>
      {key && <Keys k={key} className={css.keys} />}
    </button>
  );
}
