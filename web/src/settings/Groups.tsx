import { useState } from "react";
import { type Repo, type RepoGroup, showGroup } from "../api";
import { toast } from "../ui/toast";
import { SettingsGroupRow } from "./SettingsGroupRow";
import { SettingsGroups } from "./SettingsGroups";

// Groups are the workspace's groups, each shown or hidden for you: hidden
// ones are left out of what covers every repo (status, rg, debrief, sync),
// and their repos are cloned when they're first needed. What the team says
// is beside it, and Reset goes back to it.
export function Groups({ groups, repos, done }: { groups: RepoGroup[]; repos: Repo[]; done: () => void }) {
  const [busy, setBusy] = useState<string>();
  const label = new Map(repos.map((r) => [r.name, r.label]));
  const set = (g: string, hidden: boolean | null) => {
    setBusy(g);
    showGroup(g, hidden)
      .then((r) => {
        toast(r.message);
        return done();
      })
      .catch((e: Error) => toast(e.message, { tone: "danger", ms: 8000 }))
      .finally(() => setBusy(undefined));
  };
  return (
    <SettingsGroups none={groups.length === 0}>
      {groups.map((g) => (
        <SettingsGroupRow
          key={g.name}
          name={g.name}
          label={g.label}
          repos={g.repos.length ? g.repos.map((r) => label.get(r) ?? r).join(", ") : "—"}
          hidden={g.hidden}
          chosen={g.chosen}
          teamHidden={g.teamHidden}
          busy={busy === g.name}
          onSet={(hidden) => set(g.name, hidden)}
          onReset={() => set(g.name, null)}
        />
      ))}
    </SettingsGroups>
  );
}
