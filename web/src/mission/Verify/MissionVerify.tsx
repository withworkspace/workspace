import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import css from "./MissionVerify.module.css";

// MissionVerify is a mission's Verify tab laid out: the row of its
// bundles, over the one that's open (MissionVerifyPanes). On a wide window
// it fills what's under the mission's bar. done is the tab with nothing
// left to approve: what it says sits in its middle. fills is the tab
// filling what's under the mission's bar, as the mission page's does.
export function MissionVerify({ children, done, fills }: { children: ReactNode; done?: boolean; fills?: boolean }) {
  return <div className={cx(css.verify, done && css.done, fills && css.fills)}>{children}</div>;
}
