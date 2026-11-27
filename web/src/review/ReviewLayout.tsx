import type { Command } from "../keymap/commands";
import type { CSSProperties, PointerEvent, ReactNode, Ref, UIEventHandler } from "react";
import { ReviewSideBack } from "./ReviewSideBack";
import css from "./ReviewLayout.module.css";

// ReviewLayout is a review's page: the list down the side (as wide as
// it's been dragged), the way back at its top, its edge to drag, the
// pane the views and the changes are read in, and what stands over the
// pane's foot. What's opened over the page (a card, a panel, a dialog)
// comes after.
export function ReviewLayout({
  width,
  back,
  side,
  children,
  pane,
  onPaneScroll,
  onResize,
  onResetWidth,
  foot,
  over,
}: {
  width?: number; // the list's, as dragged; its own otherwise
  back?: { to: string; label: string; keys?: Command }; // the way back, the side's first row
  side: ReactNode;
  children: ReactNode; // the pane's
  pane?: Ref<HTMLDivElement>;
  onPaneScroll?: UIEventHandler<HTMLDivElement>;
  onResize: (e: PointerEvent<HTMLDivElement>) => void;
  onResetWidth: () => void;
  foot?: ReactNode;
  over?: ReactNode;
}) {
  return (
    <div className={css.review} style={width ? ({ "--files-w": `${width}px` } as CSSProperties) : undefined}>
      <aside className={css.side}>
        {back && <ReviewSideBack {...back} />}
        {side}
      </aside>
      <div className={css.resize} role="separator" aria-orientation="vertical" title="Drag to resize; double-click to reset" onPointerDown={onResize} onDoubleClick={onResetWidth} />
      <div className={css.pane} ref={pane} onScroll={onPaneScroll}>
        {children}
      </div>
      {foot && <div className={css.foot}>{foot}</div>}
      {over}
    </div>
  );
}
