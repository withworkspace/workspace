import type { ReactNode } from "react";
import css from "./PageFindingSuggest.module.css";
import { Text } from "../ui/Text";

// PageFindingSuggest is the words a finding proposes in place of those it's
// about.
export function PageFindingSuggest({ children }: { children: ReactNode }) {
  return (
    <div className={css.suggest}>
      <Text dim className={css.label}>Should read</Text>
      {children}
    </div>
  );
}
