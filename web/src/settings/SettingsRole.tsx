import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { toneClass } from "../ui/tone";
import css from "./SettingsRole.module.css";
import { SettingsWhat } from "./SettingsWhat";

type SettingsRoleProps = {
  title: string;
  // Its table in the file: crew, judge.
  id: string;
  about?: string;
  // Why it can't run, if it can't.
  problem?: string;
  // What it follows where it isn't set itself.
  note?: string;
  // Its agent's and its model's cells, each a control and whose it is.
  agent?: ReactNode;
  model?: ReactNode;
};

// SettingsRole is a role in the roles' table: what it is, the agent CLI it
// runs in, and the model it's given.
export function SettingsRole({ title, id, about, problem, note, agent, model }: SettingsRoleProps) {
  return (
    <div className={css.role}>
      <SettingsWhat className={css.what} label={title} tomlKey={<>[{id}]</>} about={about}>
        {problem && (
          <div className={cx(css.note, css.problem, toneClass("danger"))}>
            <Icon name="alert" size={12} /> {problem}
          </div>
        )}
        {note && <div className={css.note}>{note}</div>}
      </SettingsWhat>
      {agent ?? <span />}
      {model ?? <span />}
    </div>
  );
}
