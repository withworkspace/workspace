import type { ReactNode } from "react";
import { TypingDots } from "../ui/TypingDots";
import css from "./AnnotateTyping.module.css";

// AnnotateTyping is an agent at it in a thread: its dots, and what it's
// doing (answering, drafting what was decided).
export function AnnotateTyping({ children }: { children: ReactNode }) {
  return (
    <div className={css.typing}>
      <TypingDots />
      {children}
    </div>
  );
}
