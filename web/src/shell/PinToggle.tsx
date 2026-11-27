import type { KeyLike } from "../keymap/keymap";
import { Button } from "../ui/Button";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import css from "./PinToggle.module.css";

type PinToggleProps = {
  // Whether it's pinned now.
  on: boolean;
  // The pin alone, its words in its title: where there's no room for more.
  compact?: boolean;
  // Kept to the end of the line it's in (a ref card's foot, an artifact's
  // title line).
  end?: boolean;
  // The key that does the same, where there's one (. on a page, ⌘. in
  // Find): said in its title and hinted at (ui/keyHint.ts).
  keys?: KeyLike;
  onToggle: () => void;
};

// PinToggle pins a thing, or unpins it: Pin, and Pinned in the accent
// while it is. It's a ghost Button saying aria-pressed rather than a
// ToggleButton, whose pressed look has a fill this hasn't.
export function PinToggle({ on, compact, end, keys, onToggle }: PinToggleProps) {
  return (
    <Button
      variant="ghost"
      size="small"
      className={cx(css.pin, on && css.on, compact && css.compact, end && css.end)}
      onClick={(e) => {
        e.preventDefault();
        e.stopPropagation();
        onToggle();
      }}
      title={on ? "Remove from Focus" : "Add to Focus"}
      keys={keys}
      aria-pressed={on}
      aria-label={compact ? (on ? "Pinned" : "Pin") : undefined}
    >
      <Icon name="pin" size={12} className={css.icon} />
      {!compact && (on ? "Pinned" : "Pin")}
    </Button>
  );
}
