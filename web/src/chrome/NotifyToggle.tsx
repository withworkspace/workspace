import { Button } from "../ui/Button";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import css from "./NotifyToggle.module.css";

// NotifyToggle is whether the desktop is told when something needs you,
// beside Needs you's heading: the same switch Settings has.
export function NotifyToggle({
  on,
  onToggle,
}: {
  on: boolean;
  onToggle: () => void;
}) {
  return (
    <Button
      variant="ghost"
      size="small"
      square
      className={cx(on && css.on)}
      onClick={onToggle}
      title={
        on
          ? "Desktop notifications are on: turn them off"
          : "Notify me on the desktop when something needs me"
      }
      aria-pressed={on}
    >
      <Icon name="bell" size={12} />
    </Button>
  );
}
