import { cx } from "../ui/cx";
import css from "./AnnotateChangeBar.module.css";

// AnnotateChangeBar marks, in the document's left margin, what's changed
// since you last commented (removed: lines only taken out, a mark where
// they were). Pressed, it opens what it said then (on).
export function AnnotateChangeBar({ top, height, removed, on, onPress }: { top: number; height: number; removed?: boolean; on?: boolean; onPress: () => void }) {
  return (
    <button
      className={cx(css.bar, removed && css.removed, on && css.on)}
      style={{ top, height }}
      title={removed ? "Removed since you last commented" : "Changed since you last commented"}
      onClick={onPress}
    />
  );
}
