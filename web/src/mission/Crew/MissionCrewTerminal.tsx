import type { ReactNode } from "react";
import { Icon } from "../../ui/Icon";
import { Keys } from "../../ui/Keys";
import { cx } from "../../ui/cx";
import css from "./MissionCrewTerminal.module.css";
import { Text } from "../../ui/Text";

// MissionCrewTerminal is the frame round a crew's terminal, dark in either
// theme, and the bar under it: while it's live, that typing goes to crew
// while it has focus; once it's ended, that it's its last screen.
export function MissionCrewTerminal({ id, state, children }: { id: string; state?: "live" | "ended"; children: ReactNode }) {
  return (
    <div className={css.frame}>
      {children}
      {state === "live" && (
        <div className={css.bar}>
          <span className={css.focus}>
            <Icon name="keyboard" size={12} /> Typing goes to {id} while the terminal has focus, <Keys k="esc" /> included. Click outside to stop.
          </span>
          <Text dim>
            <Keys k="mod+C" /> copies, <Keys k="mod+V" /> pastes
          </Text>
        </div>
      )}
      {state === "ended" && (
        <div className={cx(css.bar, css.ended)}>
          <Icon name="info" size={12} /> {id} has ended: this is its last screen.
        </div>
      )}
    </div>
  );
}
