import type { ReactNode } from "react";
import css from "./TalkSide.module.css";

// TalkSide is what's come of a conversation, in a column beside it: its
// TalkSideSections, or a TalkSideEmpty while there's nothing yet.
export function TalkSide({ children }: { children: ReactNode }) {
  return (
    <aside className={css.side} aria-label="Conversation details">
      {children}
    </aside>
  );
}
