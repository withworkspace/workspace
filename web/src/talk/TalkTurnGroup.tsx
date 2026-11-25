import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./TalkTurnGroup.module.css";

// TalkTurnGroup is a turn and the proposals that came with it; settled,
// it's ws's line for a card you settled.
export function TalkTurnGroup({ settled, children }: { settled?: boolean; children: ReactNode }) {
  return <div className={cx(css.group, settled && css.settled)}>{children}</div>;
}
