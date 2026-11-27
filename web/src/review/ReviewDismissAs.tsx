import { Keys } from "../ui/Keys";
import type { DismissWhy } from "../api";
import { useCommand, useCommands } from "../keymap/useCommands";
import { reviewDismissKeys } from "./keys";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { reviewDismissWhys } from "./findings";
import css from "./ReviewDismissAs.module.css";
import { Text } from "../ui/Text";

// ReviewDismissAs asks why a finding is dismissed: 1, 2 or 3, or a click;
// esc doesn't dismiss it. While it asks, the page's keys are its own.
export function ReviewDismissAs({ onDismiss, onCancel }: { onDismiss: (why: DismissWhy) => void; onCancel: () => void }) {
  useCommands(
    reviewDismissKeys,
    {
      wrong: () => onDismiss("wrong"),
      already: () => onDismiss("already"),
      minor: () => onDismiss("minor"),
    },
    { modal: true },
  );
  const cancel = useCommand(reviewDismissKeys.cancel, onCancel, { modal: true });
  return (
    <span className={css.as} role="group" aria-label="Dismiss as" onClick={(e) => e.stopPropagation()}>
      <Text dim>Dismiss as</Text>
      {reviewDismissWhys.map(([why, label, title]) => (
        <Button variant="ghost" size="small" key={why} title={title} onClick={() => onDismiss(why)} keys={reviewDismissKeys[why]}>
          {label} <Keys k={reviewDismissKeys[why]} />
        </Button>
      ))}
      <Button variant="ghost" size="small" square onClick={cancel} aria-label="Cancel" title="Cancel">
        <Icon name="x" size={11} />
      </Button>
    </span>
  );
}
