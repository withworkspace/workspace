import { useState } from "react";
import { groupRepo, type Repo } from "../api";
import { toast } from "../ui/toast";
import { SettingsGroupPick } from "./SettingsGroupPick";
import { SettingsRepoGroup } from "./SettingsRepoGroup";

// RepoGroupCell is a repo's group in the repos table, changed for you in
// ws.local.toml: a repo of yours in any or none, one of the team's in any,
// or back in the team's.
export function RepoGroupCell({ repo, groups, labels, done }: { repo: Repo; groups: string[]; labels?: Map<string, string>; done: () => void }) {
  const [busy, setBusy] = useState(false);
  const team = repo.yours ? undefined : repo.groupYours ? repo.teamGroup || undefined : repo.group || undefined;
  const pick = (group: string) => {
    setBusy(true);
    groupRepo(repo.name, group)
      .then((r) => {
        toast(r.message);
        return done();
      })
      .catch((e: Error) => toast(e.message, { tone: "danger", ms: 8000 }))
      .finally(() => setBusy(false));
  };
  return (
    <SettingsRepoGroup over={!repo.yours && repo.groupYours ? (repo.teamGroup ? `The team has it in ${repo.teamGroup}` : "The team has it in no group") : undefined}>
      <SettingsGroupPick value={repo.group ?? ""} groups={groups} labels={labels} team={team} none={repo.yours || !team} label={`${repo.name}'s group`} onPick={pick} disabled={busy} />
    </SettingsRepoGroup>
  );
}
