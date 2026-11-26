import type { ReactNode } from "react";
import css from "./MissionLeadRequest.module.css";
import { Text } from "../../ui/Text";

// MissionLeadRequest is what was asked for, under a mission's lead while
// it's to be briefed.
export function MissionLeadRequest({ children }: { children: ReactNode }) {
  return <Text as="div" dim className={css.request}>{children}</Text>;
}
