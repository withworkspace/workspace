import type { ReactNode } from "react";
import css from "./TalkTurns.module.css";

// TalkTurns is a conversation's turns, down the page.
export function TalkTurns({ children }: { children: ReactNode }) {
  return <div className={css.turns}>{children}</div>;
}
