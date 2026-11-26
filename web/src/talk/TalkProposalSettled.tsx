import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import { Icon, type IconName } from "../ui/Icon";
import css from "./TalkProposalSettled.module.css";
import { textClass } from "../ui/Text";

// TalkProposalSettled is what came of a proposal, a line in its foot:
// done (in green: made, recorded, started), or quietly what didn't happen
// (declined, superseded, withdrawn), after its icon.
export function TalkProposalSettled({ done, icon, size = 13, children }: { done?: boolean; icon: IconName; size?: number; children: ReactNode }) {
  return (
    <span className={cx(css.line, done ? css.done : textClass("dim"))}>
      <Icon name={icon} size={size} />
      {children}
    </span>
  );
}
