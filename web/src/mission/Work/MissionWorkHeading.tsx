import type { ReactNode } from "react";
import { Count } from "../../ui/Count";
import { cx } from "../../ui/cx";
import css from "./MissionWorkHeading.module.css";

// MissionWorkHeading heads a part of what's open in the Work tab (Crew
// here, Commits, Changed files), with how many there are; end is what's
// at its right (a field to filter by). It stays at the top of its column
// as that scrolls, in a tab that fills what's under the mission's bar
// (fills).
export function MissionWorkHeading({ children, count, end, fills }: { children: ReactNode; count?: number; end?: ReactNode; fills?: boolean }) {
  return (
    <h3 className={cx(css.heading, fills && css.fills)}>
      {children} {count !== undefined && <Count>{count}</Count>}
      {end !== undefined && (
        <>
          <span className={css.spacer} />
          {end}
        </>
      )}
    </h3>
  );
}
