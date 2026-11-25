import type { ReactNode } from "react";
import css from "./RunPanelBody.module.css";

// RunPanelBody is what's shown of a run, under its RunPanelViews, taking
// what's left of the panel's height: the frame (a RunPreview) or all it's
// printed (a RunOutput, which takes runPanelLogClass). id is what the
// tabs say they control; tabpanel, whether there are tabs.
export function RunPanelBody({ id, tabpanel, label, children }: { id: string; tabpanel: boolean; label: string; children: ReactNode }) {
  return (
    <div id={id} role={tabpanel ? "tabpanel" : undefined} aria-label={label} className={css.body}>
      {children}
    </div>
  );
}

// runPanelLogClass is the look of what a run printed, filling its body.
export const runPanelLogClass = css.log;

// runPanelWaitingLogClass is the look of what a service printed while
// its frame waits for it to answer.
export const runPanelWaitingLogClass = css.waitingLog;
