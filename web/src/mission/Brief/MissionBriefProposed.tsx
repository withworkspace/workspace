import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import { Icon, type IconName } from "../../ui/Icon";
import css from "./MissionBriefProposed.module.css";

type MissionBriefProposedProps = {
  icon: IconName;
  // What was proposed: a draft of the brief, the plan.
  name: ReactNode;
  // What came of it.
  fate: ReactNode;
  // It's the one the documents show: pressing it goes to them.
  shown?: boolean;
  // It didn't stand (replaced, declined, withdrawn): its name is struck.
  struck?: boolean;
  onGo?: () => void;
};

// MissionBriefProposed is what the planner proposed for the brief or the
// plan, in its conversation: a line, not a card, since the documents show
// it. The one they show leads back to them.
export function MissionBriefProposed({ icon, name, fate, shown, struck, onGo }: MissionBriefProposedProps) {
  return (
    <button type="button" className={cx(css.proposed, shown && css.shown, struck && css.struck)} onClick={shown ? onGo : undefined} disabled={!shown} title={shown ? "It's what the documents show: go to them" : undefined}>
      <Icon name={icon} size={13} />
      <span className={css.name}>{name}</span>
      <span className={css.fate}>{fate}</span>
      {shown && <Icon name="arrow-left" size={12} />}
    </button>
  );
}
