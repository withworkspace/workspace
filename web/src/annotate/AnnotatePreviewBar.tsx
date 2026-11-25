import { Callout } from "../ui/Callout";
import { Keys } from "../ui/Keys";
import css from "./AnnotatePreviewBar.module.css";

// AnnotatePreviewBar is said over the suggested edits (count of them)
// while they're shown in place, as tracked changes: how they're drawn,
// and the keys that settle the one marked (marked, its thread's id).
export function AnnotatePreviewBar({ count, marked }: { count: number; marked: string }) {
  return (
    <Callout tone="info" icon="eye" title={count === 1 ? "The suggested edit, in place" : `${count} suggested edits, in place`} className={css.bar}>
      Struck through is what an edit takes out, underlined what it puts in. <Keys k="a" /> accepts the one marked ({marked}), <Keys k="d" /> declines it; click another to mark it.{" "}
      <Keys k="v" /> or <Keys k="esc" /> goes back.
    </Callout>
  );
}
