import type { ReactNode } from "react";
import css from "./MissionBriefDocNote.module.css";
import { Text } from "../../ui/Text";

// MissionBriefDocNote is a quiet line over a document in the Brief tab:
// a change that couldn't be shown in place, the form ws holds a plan to.
export function MissionBriefDocNote({ children }: { children: ReactNode }) {
  return <Text as="p" dim className={css.note}>{children}</Text>;
}
