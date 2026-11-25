import type { ReactNode } from "react";
import css from "./TalkSideNamed.module.css";

// TalkSideNamed is a list of short things beside a conversation, a few to
// a line: its children are its <li>s. refs: they're refs, which say so
// (data-refs) for their look.
export function TalkSideNamed({ refs, children }: { refs?: boolean; children: ReactNode }) {
  return (
    <ul className={css.named} data-refs={refs ? "" : undefined}>
      {children}
    </ul>
  );
}
