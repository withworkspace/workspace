import type { DismissWhy } from "../api";
import { useKeys } from "../keys";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import { reviewDismissWhys } from "./findings";
import css from "./ReviewDismissAs.module.css";
import { Text } from "../ui/Text";

// ReviewDismissAs asks why a finding is dismissed: 1, 2 or 3, or a click;
// esc doesn't dismiss it. While it asks, the page's keys are its own.
export function ReviewDismissAs({ onDismiss, onCancel }: { onDismiss: (why: DismissWhy) => void; onCancel: () => void }) {
  useKeys(
    {
      "1": () => onDismiss("wrong"),
      "2": () => onDismiss("already"),
      "3": () => onDismiss("minor"),
      Escape: onCancel,
    },
    { modal: true },
  );
  return (
    <span className={css.as} role="group" aria-label="Dismiss it as" onClick={(e) => e.stopPropagation()}>
      <Text dim>Dismiss as</Text>
      {reviewDismissWhys.map(([why, label, title], i) => (
        <Button variant="ghost" size="small" key={why} title={title} onClick={() => onDismiss(why)} keys={`${i + 1}`}>
          {label} <Keys k={`${i + 1}`} />
        </Button>
      ))}
      <Button variant="ghost" size="small" square onClick={onCancel} aria-label="Don't dismiss it" title="Don't dismiss it" keys="esc">
        <Icon name="x" size={11} />
      </Button>
    </span>
  );
}
