import type { DismissWhy } from "../api";
import { Button } from "../ui/Button";
import { Keys } from "../ui/Keys";
import { ReviewDismissAs } from "./ReviewDismissAs";
import css from "./ReviewFindingActions.module.css";

// ReviewFindingActions are what's done with an open finding, as buttons:
// Take, Dismiss (which then asks why) and Ask. Where the cursor's on it,
// they say their keys; where there's no cursor (the Passes view's list),
// they have none.
export function ReviewFindingActions({
  dismissing,
  cursor,
  canAsk,
  onTake,
  onDismissing,
  onDismiss,
  onAsk,
}: {
  dismissing: boolean;
  cursor?: boolean; // the cursor's on it: say its keys
  canAsk?: boolean;
  onTake: () => void;
  onDismissing: (on: boolean) => void;
  onDismiss: (why: DismissWhy) => void;
  onAsk?: () => void;
}) {
  if (dismissing) return <ReviewDismissAs onDismiss={onDismiss} onCancel={() => onDismissing(false)} />;
  const keyed = cursor !== undefined;
  return (
    <span className={css.actions} onClick={(e) => e.stopPropagation()}>
      <Button variant="primary" size="small" onClick={onTake} title="Put it in your draft review as a comment, marked as the agent's: yours to edit, and posted only when you submit" keys={keyed ? "⏎" : undefined}>
        Take {cursor && <Keys k="⏎" />}
      </Button>
      <Button variant="ghost" size="small" onClick={() => onDismissing(true)} title="It isn't one: don't raise it again. You'll say why" keys={keyed ? "D" : undefined}>
        Dismiss {cursor && <Keys k="D" />}
      </Button>
      {canAsk && onAsk && (
        <Button variant="ghost" size="small" onClick={onAsk} title="Ask an agent about it, with the code checked out here" keys={keyed ? "a" : undefined}>
          Ask {cursor && <Keys k="a" />}
        </Button>
      )}
    </span>
  );
}
