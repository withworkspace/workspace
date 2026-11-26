import type { ReactNode } from "react";
import { SettingsAbout } from "./SettingsAbout";
import css from "./SettingsGroups.module.css";
import { SettingsTable } from "./SettingsTable";

type SettingsGroupsProps = {
  // There are none yet: said, with how one's made.
  none?: boolean;
  // A SettingsGroupRow each.
  children?: ReactNode;
};

// SettingsGroups is the workspace's groups under its repos, each shown or
// hidden for you, with what the team says beside it.
export function SettingsGroups({ none, children }: SettingsGroupsProps) {
  return (
    <div className={css.groups}>
      <h3 className={css.head}>Groups</h3>
      {none ? (
        <SettingsAbout>None yet: set a repo's group from its row. Hidden groups are left out of commands that cover every repo, and cloned when first needed.</SettingsAbout>
      ) : (
        <>
          <SettingsAbout>Hidden groups are left out of commands that cover every repo (status, rg, debrief, sync), and cloned when first needed. Shown or hidden is your setting, in ws.local.toml.</SettingsAbout>
          <SettingsTable>
            <thead>
              <tr>
                <th>Group</th>
                <th>Repos</th>
                <th>For you</th>
                <th aria-label="Team setting" />
              </tr>
            </thead>
            <tbody>{children}</tbody>
          </SettingsTable>
        </>
      )}
    </div>
  );
}
