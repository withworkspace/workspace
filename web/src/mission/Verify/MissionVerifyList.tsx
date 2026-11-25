import type { ReactNode } from "react";
import css from "./MissionVerifyList.module.css";

// MissionVerifyList is the open bundle's checks, in groups
// (MissionVerifyGroup) in the order your attention's needed, with the
// bundle's approval (MissionVerifyApprove) among them.
export function MissionVerifyList({ children }: { children: ReactNode }) {
  return <div className={css.list}>{children}</div>;
}
