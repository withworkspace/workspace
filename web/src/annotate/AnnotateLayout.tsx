import type { ComponentProps, CSSProperties, ReactNode, Ref } from "react";
import { cx } from "../ui/cx";
import css from "./AnnotateLayout.module.css";

// AnnotateLayout is a document being commented on: its column (an
// AnnotateDocCol, with the document as an AnnotateText and what's drawn
// over it) and its margin (an AnnotateMargin of AnnotateSlots, a thread's
// card in each). On a narrow window there's no margin: it's the column
// alone. hidden keeps it, unseen, while something else is shown in its
// place (the suggested edits, in place).
export function AnnotateLayout({ hidden, children }: { hidden?: boolean; children: ReactNode }) {
  return (
    <div className={css.annotate} style={hidden ? { display: "none" } : undefined}>
      {children}
    </div>
  );
}

// AnnotateDocCol is the document's column: what's drawn over the document
// (the cursor, marks, bars) is placed in it.
export function AnnotateDocCol({ ref, children }: { ref?: Ref<HTMLDivElement>; children: ReactNode }) {
  return (
    <div className={css.col} ref={ref}>
      {children}
    </div>
  );
}

// AnnotateText is the document, as it's read, in its column.
export function AnnotateText({ className, ...rest }: ComponentProps<"article">) {
  return <article className={cx("artifact-body", css.text, className)} {...rest} />;
}

// AnnotateMargin is the margin beside the document, as tall as what's in
// it is placed.
export function AnnotateMargin({ height, children }: { height: number; children: ReactNode }) {
  return (
    <div className={css.margin} style={{ height }}>
      {children}
    </div>
  );
}

// AnnotateSlot is a card's place in the margin, level with what it's on:
// see-through until it's placed (top undefined), not hidden, so what's in
// it can take the focus.
export function AnnotateSlot({ ref, top, children }: { ref?: Ref<HTMLDivElement>; top?: number; children: ReactNode }) {
  const style: CSSProperties = { top: top ?? 0, opacity: top === undefined ? 0 : undefined };
  return (
    <div ref={ref} className={css.slot} style={style}>
      {children}
    </div>
  );
}

// annotateLandedClass is what's put on a passage of the document for a
// moment, as an edit just accepted lands in it.
export const annotateLandedClass = css.landed;
