import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { hintOnClick } from "../ui/keyHint";
import { Keys } from "../ui/Keys";
import css from "./FindButton.module.css";

// FindButton is the sidebar's way to Find: what it's called, and its key.
// In the rail it's its icon.
export function FindButton({
  onPress,
  className,
}: {
  onPress?: () => void;
  className?: string;
}) {
  return (
    <button
      type="button"
      data-ui=""
      aria-label="Find"
      className={cx(css.find, className)}
      onClick={hintOnClick("mod+K", onPress)}
    >
      <Icon name="search" size={14} />
      <span className={css.words}>Find…</span>
      <Keys k="mod+K" className={css.keys} />
    </button>
  );
}
