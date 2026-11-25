import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import { Icon } from "../../ui/Icon";
import css from "./MissionLogCrew.module.css";

// MissionLogCrew is one of the crew the Log tab lists (those who worked in
// none of the mission's capsules): its row, and a MissionLogHandoff where
// it left one.
export function MissionLogCrew({ children }: { children: ReactNode }) {
  return <div className={css.crew}>{children}</div>;
}

// MissionLogHandoff opens what a crew handed over when it ended. id is
// what the page's keys find it by; selected, the keys' cursor is on it.
export function MissionLogHandoff({ id, selected, onOpen, children }: { id: string; selected?: boolean; onOpen: () => void; children: ReactNode }) {
  return (
    <button id={id} className={cx(css.handoff, selected && css.selected)} onClick={onOpen}>
      <Icon name="arrow-right" size={13} />
      {children}
    </button>
  );
}
