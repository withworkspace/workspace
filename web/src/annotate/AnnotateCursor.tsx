import css from "./AnnotateCursor.module.css";

// AnnotateCursor is the line beside the paragraphs the keys are on (j, k),
// which c comments on: top and height in the document's column.
export function AnnotateCursor({ top, height }: { top: number; height: number }) {
  return <div className={css.cursor} style={{ top, height }} />;
}
