import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import css from "./MissionSummaryWork.module.css";
import { Text } from "../../ui/Text";

// MissionSummaryWork is what a mission's Summary says of its Work tab in a
// line, under the capsules in hand (afterCards, when there are some).
export function MissionSummaryWork({ afterCards, children }: { afterCards?: boolean; children: ReactNode }) {
  return <Text dim className={cx(css.work, afterCards && css.afterCards)}>{children}</Text>;
}
