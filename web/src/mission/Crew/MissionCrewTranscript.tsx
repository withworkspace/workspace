import type { ReactNode } from "react";
import css from "./MissionCrewTranscript.module.css";

// MissionCrewTranscript is crew's transcript: the same turns as a
// conversation, one under another.
export function MissionCrewTranscript({ children }: { children: ReactNode }) {
  return <div className={css.transcript}>{children}</div>;
}
