import { cx } from "../ui/cx";
import css from "./AnnotateBlockMark.module.css";

// AnnotateBlockMark is drawn round a thing in the document with no words
// to highlight (a diagram) that a thread is on; on, while that thread's
// in focus, pointed at or being written.
export function AnnotateBlockMark({ top, left, width, height, on }: { top: number; left: number; width: number; height: number; on?: boolean }) {
  return <div className={cx(css.mark, on && css.on)} style={{ top, height, left, width }} />;
}
