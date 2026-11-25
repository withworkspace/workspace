import type { ReactNode } from "react";
import css from "./RunPanelFrame.module.css";

// RunPanelFrame is one run of a place's, or a recipe of its, as its Runs
// tab shows the one picked (label says which, and where it stands): a
// RunPanelHead, its RunPanelFacts, then a RunPanelViews over a
// RunPanelBody (or a RunPanelIdle), which takes what's left of the height.
export function RunPanelFrame({ label, children }: { label: string; children: ReactNode }) {
  return (
    <section className={css.panel} aria-label={label}>
      {children}
    </section>
  );
}
