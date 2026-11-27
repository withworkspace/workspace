import { Keys } from "../ui/Keys";
import { reviewKeys } from "./keys";
import type { DismissWhy } from "../api";
import { Button } from "../ui/Button";
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
      <Button variant="primary" size="small" onClick={onTake} title="Add to your draft review as a comment. Posted when you submit" keys={keyed ? reviewKeys.activate : undefined}>
        Take {cursor && <Keys k={reviewKeys.activate} />}
      </Button>
      <Button variant="ghost" size="small" onClick={() => onDismissing(true)} title="Not an issue: stop raising it" keys={keyed ? reviewKeys.remove : undefined}>
        Dismiss {cursor && <Keys k={reviewKeys.remove} />}
      </Button>
      {canAsk && onAsk && (
        <Button variant="ghost" size="small" onClick={onAsk} title="Ask an agent about this" keys={keyed ? reviewKeys.ask : undefined}>
          Ask {cursor && <Keys k={reviewKeys.ask} />}
        </Button>
      )}
    </span>
  );
}
