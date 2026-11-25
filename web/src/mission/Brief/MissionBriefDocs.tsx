import type { ReactNode, Ref } from "react";
import { cx } from "../../ui/cx";
import css from "./MissionBriefDocs.module.css";

type MissionBriefDocsProps = {
  ref?: Ref<HTMLDivElement>;
  // Side by side: there's room for the brief and the plan each at a column
  // worth reading, and each has the tab's height.
  side?: boolean;
  children: ReactNode;
};

// MissionBriefDocs holds the brief and the plan (a MissionBriefDoc each,
// with a MissionBriefDocsNote where something's to be said of them): one
// under the other, scrolling together, or side by side.
export function MissionBriefDocs({ ref, side, children }: MissionBriefDocsProps) {
  return (
    <div className={cx(css.docs, side && css.side)} ref={ref}>
      {children}
    </div>
  );
}
