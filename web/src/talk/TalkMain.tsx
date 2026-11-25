import type { ReactNode } from "react";
import css from "./TalkMain.module.css";

// TalkMain is the conversation itself, under its bar: what it was asked
// of, its turns, and the reply box at its foot.
export function TalkMain({ children }: { children: ReactNode }) {
  return <div className={css.main}>{children}</div>;
}
