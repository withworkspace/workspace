import type { ReactNode } from "react";
import css from "./MissionWorkCrew.module.css";

// MissionWorkCrew is who's been in a capsule: a MissionWorkCrewLine each.
export function MissionWorkCrew({ children }: { children: ReactNode }) {
  return <div className={css.crew}>{children}</div>;
}
