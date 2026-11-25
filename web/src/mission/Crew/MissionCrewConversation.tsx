import type { ReactNode } from "react";
import css from "./MissionCrewConversation.module.css";

// MissionCrewConversation is crew as a conversation: its plan pinned on
// top, then what's said of its transcript (none to show) and the
// transcript, and at the bottom, docked, what it asks and a box to write
// to it.
export function MissionCrewConversation({ plan, note, children, dock }: { plan?: ReactNode; note?: ReactNode; children: ReactNode; dock: ReactNode }) {
  return (
    <div className={css.view}>
      {plan}
      {note}
      {children}
      {dock}
    </div>
  );
}
