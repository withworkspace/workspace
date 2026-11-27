import { annotateKeys } from "./keys";
import { Callout } from "../ui/Callout";
import { Keys } from "../ui/Keys";
import css from "./AnnotatePreviewBar.module.css";

// AnnotatePreviewBar is said over the suggested edits (count of them)
// while they're shown in place, as tracked changes: how they're drawn,
// and the keys that settle the one marked (marked, its thread's id).
export function AnnotatePreviewBar({ count, marked }: { count: number; marked: string }) {
  return (
    <Callout tone="info" icon="eye" title={count === 1 ? "Suggested edit" : `${count} suggested edits`} className={css.bar}>
      <Keys k={annotateKeys.accept} /> accepts the selected edit ({marked}), <Keys k={annotateKeys.decline} /> declines. Click another to select
      it. <Keys k={annotateKeys.tracked} /> or <Keys k={annotateKeys.back} /> to go back.
    </Callout>
  );
}
