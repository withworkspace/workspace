import type { ReactNode } from "react";
import { SettingsTable } from "./SettingsTable";

// SettingsReposTable is the workspace's repos as a table, a SettingsRepoRow
// each. acts is whether any has something to be done to it (one of yours:
// given to the team), which has a column of its own.
export function SettingsReposTable({ acts, children }: { acts?: boolean; children: ReactNode }) {
  return (
    <SettingsTable>
      <thead>
        <tr>
          <th>Repo</th>
          <th>Name</th>
          <th>On GitHub</th>
          <th>Aliases</th>
          <th>Group</th>
          <th>Lands by</th>
          {acts && <th aria-label="Actions" />}
        </tr>
      </thead>
      <tbody>{children}</tbody>
    </SettingsTable>
  );
}
