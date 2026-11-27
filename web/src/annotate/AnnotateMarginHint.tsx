import { annotateKeys } from "./keys";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import css from "./AnnotateMarginHint.module.css";

// AnnotateMarginHint is what the margin says while there are no threads
// in it: how to comment, and what comes of it.
export function AnnotateMarginHint() {
  return (
    <div className={css.hint}>
      <Icon name="message" size={14} className={css.icon} />
      <span>
        Select text to comment, or move with <Keys k={annotateKeys.next} /> <Keys k={annotateKeys.prev} /> and press <Keys k={annotateKeys.comment} />. Ask a
        question, request a change, or leave a note: an agent replies in the thread.
      </span>
    </div>
  );
}
