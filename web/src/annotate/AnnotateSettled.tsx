import type { ReactNode } from "react";
import css from "./AnnotateSettled.module.css";
import { Text } from "../ui/Text";

// AnnotateSettled is a quiet line of what's come of a thread: an edit
// accepted or declined, a decision recorded, crew finished.
export function AnnotateSettled({ children }: { children: ReactNode }) {
  return <Text as="div" dim className={css.settled}>{children}</Text>;
}
