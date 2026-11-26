import type { ReactNode } from "react";
import css from "./MissionBriefDocWaiting.module.css";
import { Text } from "../../ui/Text";

// MissionBriefDocWaiting is what's said in a document's place while
// there's nothing in it: no plan yet, and who writes one.
export function MissionBriefDocWaiting({ children }: { children: ReactNode }) {
  return <Text as="div" dim className={css.waiting}>{children}</Text>;
}
