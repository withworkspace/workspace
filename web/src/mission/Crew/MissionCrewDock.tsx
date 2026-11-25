import type { ReactNode } from "react";
import { Icon } from "../../ui/Icon";
import css from "./MissionCrewDock.module.css";

// MissionCrewDock is what's docked at the bottom of crew's conversation:
// what it asks, and the box to write to it; over it, while there's more of
// the transcript below what's read, a pill that goes down to it.
export function MissionCrewDock({ onNewer, children }: { onNewer?: () => void; children: ReactNode }) {
  return (
    <div className={css.dock}>
      {onNewer && (
        <button className={css.pill} onClick={onNewer}>
          <Icon name="arrow-down" size={12} /> Newer
        </button>
      )}
      {children}
    </div>
  );
}
