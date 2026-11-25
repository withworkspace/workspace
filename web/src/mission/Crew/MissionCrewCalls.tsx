import type { ReactNode } from "react";
import css from "./MissionCrewCalls.module.css";

// MissionCrewCalls is where a run of crew's steps between its turns sits
// (the steps themselves are Calls): set in under the avatar, as a turn's
// words are.
export function MissionCrewCalls({ children }: { children: ReactNode }) {
  return <div className={css.calls}>{children}</div>;
}
