import { Keys } from "../ui/Keys";
import { reviewKeys } from "./keys";
import { Button } from "../ui/Button";
import css from "./ReviewSelectBar.module.css";

// ReviewSelectBar hangs under lines picked in a review: which they are,
// Comment (c) where comments can be made, and Ask (a) where an agent can
// be asked. (esc clears them, as it says in the keys.) Pressing its
// buttons keeps the focus where it was.
export function ReviewSelectBar({ label, onComment, onAsk }: { label: string; onComment?: () => void; onAsk?: () => void }) {
  return (
    <div className={css.bar}>
      <span>{label}</span>
      {onComment && (
        <Button variant="primary" onMouseDown={(e) => e.preventDefault()} onClick={onComment} keys={reviewKeys.comment}>
          Comment <Keys k={reviewKeys.comment} />
        </Button>
      )}
      {onAsk && (
        <Button onMouseDown={(e) => e.preventDefault()} onClick={onAsk} title="Ask an agent about these lines" keys={reviewKeys.ask}>
          Ask <Keys k={reviewKeys.ask} />
        </Button>
      )}
    </div>
  );
}
