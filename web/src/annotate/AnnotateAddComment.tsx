import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import css from "./AnnotateAddComment.module.css";

// AnnotateAddComment is the button that comments, in the margin level with
// what it'd be on (top): beside text selected it says so (text), beside
// the cursor's paragraphs it's a plus. It keeps the selection when pressed.
export function AnnotateAddComment({ top, text, onPress }: { top: number; text?: boolean; onPress: () => void }) {
  return (
    <button className={cx(css.add, text && css.text)} style={{ top }} onMouseDown={(e) => e.preventDefault()} onClick={onPress} title="Comment (c)">
      {text ? (
        <>
          <Icon name="message" size={13} /> Comment <Keys k="c" />
        </>
      ) : (
        <>
          <Icon name="plus" size={12} /> <Keys k="c" />
        </>
      )}
    </button>
  );
}
