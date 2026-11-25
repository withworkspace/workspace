import type { ReactNode } from "react";
import css from "./SpaceSplit.module.css";

// SpaceSplit is a space's list and, beside it, the SpaceEdge and the
// document picked: the list as wide as it's been dragged (listWidth), or
// five parts to the preview's seven.
export function SpaceSplit({ listWidth, children }: { listWidth?: number; children: ReactNode }) {
  return (
    <div className={css.split} data-space-split="" style={listWidth ? ({ "--space-list-w": `${listWidth}px` } as React.CSSProperties) : undefined}>
      {children}
    </div>
  );
}
