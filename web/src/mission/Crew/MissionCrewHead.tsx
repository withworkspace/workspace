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
  // What's done to it (Stop).
  actions?: ReactNode;
};

// MissionCrewHead is the head of a crew's page: who it is and how it
// stands, then what it runs on, where, and what its terminal calls itself.
export function MissionCrewHead({ name, status, runs, acp, capsule, termTitle, actions }: MissionCrewHeadProps) {
  return (
    <header className={css.head}>
      <div className={css.title}>
        <h1>
          {name}
          <Status s={status} className={css.status} />
        </h1>
        <div className={css.sub}>
          {runs && <span>{runs}</span>}
          {acp && <Text dim>over ACP, no terminal</Text>}
          {capsule && (
            <Text mono className={css.fact}>
              <Icon name="box" size={12} /> {capsule}
            </Text>
          )}
          {termTitle && <Text mono className={cx(css.fact, css.termTitle)} title="Terminal title">{termTitle}</Text>}
        </div>
      </div>
      {actions}
    </header>
  );
}
