import type { ReactNode } from "react";
import css from "./MissionLogMoved.module.css";
import { Text } from "../../ui/Text";

// MissionLogMoved says, under the log, where its crew are: on Work, with
// the capsule each worked in, and which are here because they worked in
// none.
export function MissionLogMoved({ children }: { children: ReactNode }) {
  return <Text as="p" dim className={css.moved}>{children}</Text>;
}
