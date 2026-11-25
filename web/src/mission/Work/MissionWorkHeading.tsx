import type { ReactNode } from "react";
import { Count } from "../../ui/Count";
import css from "./MissionWorkHeading.module.css";

// MissionWorkHeading heads a part of what's open in the Work tab (Crew
// here, Commits, Changed files), with how many there are; end is what's
// at its right (a field to filter by). It stays at the top of its column
// as that scrolls.
export function MissionWorkHeading({ children, count, end }: { children: ReactNode; count?: number; end?: ReactNode }) {
  return (
    <h3 className={css.heading}>
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
