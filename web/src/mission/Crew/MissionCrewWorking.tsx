import type { ReactNode } from "react";
import { TypingDots } from "../../ui/TypingDots";
import css from "./MissionCrewWorking.module.css";

// MissionCrewWorking is crew at work, under its transcript: dots, and
// what it's at (children: the step it's on, its file or command in code).
export function MissionCrewWorking({ children }: { children: ReactNode }) {
  return (
    <div className={css.working}>
      <TypingDots />
      {children}
    </div>
  );
}
