import type { ReactNode } from "react";
import { Icon } from "../../ui/Icon";
import css from "./MissionCrewNotice.module.css";

// MissionCrewNotice is what crew's harness put in its transcript
// (Interrupted), a line in the middle.
export function MissionCrewNotice({ children }: { children: ReactNode }) {
  return (
    <div className={css.notice}>
      <Icon name="info" size={12} /> {children}
    </div>
  );
}
