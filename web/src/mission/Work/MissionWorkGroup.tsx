import type { ReactNode } from "react";
import { GroupHead } from "../../ui/GroupHead";
import css from "./MissionWorkGroup.module.css";

// MissionWorkGroup heads a group in the Work tab's list (Capsules,
// Outputs), with how many it has.
export function MissionWorkGroup({ count, children }: { count: number; children: ReactNode }) {
  return (
    <GroupHead className={css.group} count={count}>
      {children}
    </GroupHead>
  );
}
