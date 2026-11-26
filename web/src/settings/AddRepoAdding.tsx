import { useState } from "react";
import { type AddableRepo, addRepo } from "../api";
import { toast } from "../ui/toast";
import { SettingsAddRepoForm } from "./SettingsAddRepoForm";
import { SettingsGroupPick } from "./SettingsGroupPick";

// AddRepoAdding is the repo chosen, about to be added: its name here (its
// name on GitHub, unless that's taken here), and whose it is.
export function AddRepoAdding({ owner, repo, taken, groups, groupLabels, onCancel, onAdded }: { owner: string; repo: AddableRepo; taken: Set<string>; groups: string[]; groupLabels?: Map<string, string>; onCancel: () => void; onAdded: () => void }) {
  const [as, setAs] = useState(() => (taken.has(repo.name) ? `${owner}-${repo.name}` : repo.name));
  const [group, setGroup] = useState("");
  const [team, setTeam] = useState(false);
  const [busy, setBusy] = useState(false);
  const name = as.trim();
  const clash = taken.has(name);
  const bad = !/^[A-Za-z0-9_-][A-Za-z0-9._-]*$/.test(name);
  const add = () => {
    setBusy(true);
    addRepo({ owner, name: repo.name, as: name !== repo.name ? name : undefined, group: group || undefined, team })
      .then((r) => {
        toast(r.message);
        onAdded();
      })
      .catch((e: Error) => {
        toast(e.message, { tone: "danger", ms: 8000 });
        setBusy(false);
      });
  };
  return (
    <SettingsAddRepoForm
      as={as}
      invalid={clash || bad}
      hint={clash ? `${name} already exists.` : bad ? "Letters, digits, '.', '_' and '-'." : name !== repo.name ? `Still ${owner}/${repo.name} on GitHub.` : "Folder under repos/, and name in commands."}
      onAs={setAs}
      group={<SettingsGroupPick value={group} groups={groups} labels={groupLabels} none label="Group" onPick={setGroup} />}
      team={team}
      onTeam={setTeam}
      busy={busy}
      add={busy ? "Adding…" : team ? `Add ${owner}/${repo.name} for the team` : `Add ${owner}/${repo.name}`}
      onCancel={onCancel}
      onSubmit={add}
    />
  );
}
