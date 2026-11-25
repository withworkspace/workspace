import { Icon } from "../../ui/Icon";
import { Time } from "../../ui/Time";
import { TurnAvatar } from "../../ui/Turn";
import css from "./MissionCrewBriefed.module.css";

// MissionCrewBriefed is ws's brief to crew, its opening prompt: folded to
// a line, its words as they were written when opened.
export function MissionCrewBriefed({ id, text, time }: { id: string; text: string; time?: string }) {
  return (
    <details className={css.briefed}>
      <summary>
        <Icon name="chevron-right" size={12} className={css.chevron} />
        <TurnAvatar by="agent" className={css.avatar} />
        ws briefed {id}: the mission, and how to report back
        {time && <Time iso={time} className="dim" />}
      </summary>
      <div className={css.said}>{text}</div>
    </details>
  );
}
