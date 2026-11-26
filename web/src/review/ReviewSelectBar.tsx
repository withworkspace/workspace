import { Button } from "../ui/Button";
import { Keys } from "../ui/Keys";
import css from "./ReviewSelectBar.module.css";
import { Text } from "../ui/Text";

// ReviewSelectBar hangs under lines picked in a review: which they are,
// Comment (c) where comments can be made, Ask (a) where an agent can be
// asked, and that esc clears them. Pressing its buttons keeps the focus
// where it was.
export function ReviewSelectBar({ label, onComment, onAsk }: { label: string; onComment?: () => void; onAsk?: () => void }) {
  return (
    <div className={css.bar}>
      <span>{label}</span>
      {onComment && (
        <Button variant="primary" onMouseDown={(e) => e.preventDefault()} onClick={onComment} keys="c">
          Comment <Keys k="c" />
        </Button>
      )}
      {onAsk && (
        <Button onMouseDown={(e) => e.preventDefault()} onClick={onAsk} title="Ask an agent about these lines" keys="a">
          Ask <Keys k="a" />
        </Button>
      )}
      <Text dim>
        <Keys k="esc" /> to clear
      </Text>
    </div>
  );
}
