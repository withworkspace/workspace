import type { ReactNode } from "react";
import { Icon } from "../../ui/Icon";
import { Status, type StatusInfo } from "../../ui/Status";
import { cx } from "../../ui/cx";
import css from "./MissionCrewHead.module.css";
import { Text } from "../../ui/Text";

type MissionCrewHeadProps = {
  // Who it is: its badge, or a briefing agent's name.
  name: ReactNode;
  status: StatusInfo;
  // What it runs on ("claude on sonnet", or its model's switch while it
  // runs), or that it's briefing.
  runs?: ReactNode;
  // Over ACP: no terminal.
  acp?: boolean;
  capsule?: string;
  // What its terminal calls itself.
  termTitle?: string;
  // Which of it is shown: its conversation or its terminal (a Segmented).
  view?: ReactNode;
  // What's done to it (Stop).
  actions?: ReactNode;
};

// MissionCrewHead is the bar of a crew's page, under the mission's: who it
// is and how it stands, what it runs on, where, and what its terminal
// calls itself, all in one row that stays while the page scrolls; then
// which of it is shown, and what's done to it. Its facts give way first,
// its terminal's title before the rest.
export function MissionCrewHead({ name, status, runs, acp, capsule, termTitle, view, actions }: MissionCrewHeadProps) {
  return (
    <header className={css.head}>
      <div className={css.who}>
        <h2 className={css.name}>{name}</h2>
        <Status s={status} className={css.status} />
      </div>
      <div className={css.facts}>
        {runs && <span className={css.fact}>{runs}</span>}
        {acp && <Text dim className={css.fact}>over ACP, no terminal</Text>}
        {capsule && (
          <Text mono className={css.fact} title={capsule}>
            <Icon name="box" size={12} /> {capsule}
          </Text>
        )}
        {termTitle && (
          <Text mono className={cx(css.fact, css.termTitle)} title={`Terminal title: ${termTitle}`}>
            {termTitle}
          </Text>
        )}
      </div>
      {(view || actions) && (
        <div className={css.end}>
          {view}
          {actions}
        </div>
      )}
    </header>
  );
}
