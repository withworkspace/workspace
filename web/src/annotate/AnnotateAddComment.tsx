import { annotateKeys } from "./keys";
import { keyOf } from "../keymap/keymap";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { keyProps } from "../ui/keyHint";
import { Keys } from "../ui/Keys";
import css from "./AnnotateAddComment.module.css";

// AnnotateAddComment is the button that comments, in the margin level with
// what it'd be on (top): beside text selected it says so (text), beside
// the cursor's paragraphs it's a plus. It keeps the selection when pressed.
export function AnnotateAddComment({ top, text, onPress }: { top: number; text?: boolean; onPress: () => void }) {
  return (
    <button className={cx(css.add, text && css.text)} style={{ top }} onMouseDown={(e) => e.preventDefault()} {...keyProps(keyOf(annotateKeys.comment), "Comment", () => onPress())}>
      {text ? (
        <>
          <Icon name="message" size={13} /> Comment <Keys k={annotateKeys.comment} />
        </>
      ) : (
        <>
          <Icon name="plus" size={12} /> <Keys k={annotateKeys.comment} />
        </>
      )}
    </button>
  );
}
