import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import css from "./MissionCrewPage.module.css";

// MissionCrewPage is a crew's page: a page, narrower than the widest.
export function MissionCrewPage({ children }: { children: ReactNode }) {
  return <div className={cx("page", css.page)}>{children}</div>;
}
