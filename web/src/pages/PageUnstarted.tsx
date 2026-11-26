import type { ReactNode } from "react";
import css from "./PageUnstarted.module.css";
import { Text } from "../ui/Text";

// PageUnstarted says a cell's agent hasn't started, or couldn't, and
// offers to try again (children).
export function PageUnstarted({ words, children }: { words: ReactNode; children: ReactNode }) {
  return (
    <div className={css.unstarted}>
      <Text dim>{words}</Text>
      {children}
    </div>
  );
}
