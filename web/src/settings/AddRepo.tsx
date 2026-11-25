import { useEffect, useMemo, useState } from "react";
import { type AddableRepo, getGitHubOwners, getGitHubRepos } from "../api";
import { useLoaded } from "../load";
import { Callout } from "../ui/Callout";
import { Dialog } from "../ui/Dialog";
import { relative } from "../ui/when";
import { AddRepoAdding } from "./AddRepoAdding";
import { SettingsAddRepoAll } from "./SettingsAddRepoAll";
import { SettingsAddRepoBar } from "./SettingsAddRepoBar";
import { SettingsAddRepoList } from "./SettingsAddRepoList";
import { SettingsAddRepoOption } from "./SettingsAddRepoOption";

// AddRepo picks a repo from GitHub and adds it to the workspace: whose
// (you, or an org you're in, or another named), which of theirs (newest
// pushed first, forks and archived ones hidden unless asked for), its
// name here, and whether it's yours alone or the team's. What GitHub said
// is shown at once, however old, and again once it's been asked: seen
// moves when it has (the github topic, followed by the page).
export function AddRepo({ seen, org, groups, groupLabels, taken, onClose, onAdded }: { seen: number; org?: string; groups: string[]; groupLabels?: Map<string, string>; taken: Set<string>; onClose: () => void; onAdded: () => void }) {
  const owners = useLoaded(() => getGitHubOwners(), []);
  const [picked, setPicked] = useState<string>();
  const [typed, setTyped] = useState("");
  const [another, setAnother] = useState(false);
  const list = owners.value?.owners ?? [];
  const owner = another ? typed.trim() : (picked ?? list.find((o) => o.login.toLowerCase() === org?.toLowerCase())?.login ?? list[0]?.login ?? "");
  const repos = useLoaded(owner ? () => getGitHubRepos(owner) : null, [owner], { keep: true });
  // A look behind finished, or a clone: what's shown is asked for again.
  useEffect(() => {
    if (seen === 0) return;
    owners.reload();
    if (owner) repos.reload();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [seen]);

  const [q, setQ] = useState("");
  const [all, setAll] = useState(false);
  const [chosen, setChosen] = useState<AddableRepo>();
  const shown = useMemo(() => {
    const got = repos.value?.owner.toLowerCase() === owner.toLowerCase() ? repos.value.repos : [];
    const words = q.trim().toLowerCase();
    return got.filter((r) => (all || (!r.fork && !r.archived)) && (!words || `${r.name} ${r.description ?? ""}`.toLowerCase().includes(words)));
  }, [repos.value, owner, q, all]);
  const hidden = (repos.value?.repos ?? []).filter((r) => r.fork || r.archived).length;
  const looking = repos.value?.looking || owners.value?.looking;
  const at = repos.value?.at;

  const look = () => {
    getGitHubOwners(true).catch(() => undefined);
    if (owner) getGitHubRepos(owner, true).then(repos.set, () => undefined);
  };

  return (
    <Dialog title="Add a repo from GitHub" onClose={onClose} wide>
      <SettingsAddRepoBar
        owners={list}
        owner={owner}
        another={another}
        naming={another || (!!owners.value?.problem && list.length === 0)}
        named={typed}
        filter={q}
        when={looking ? "asking GitHub…" : at ? `as of ${relative(at)}` : ""}
        looking={looking}
        onOwner={(login) => {
          setChosen(undefined);
          setAnother(false);
          setPicked(login);
        }}
        onAnother={() => {
          setChosen(undefined);
          setAnother(true);
        }}
        onNamed={(text) => {
          setAnother(true);
          setTyped(text);
          setChosen(undefined);
        }}
        onFilter={setQ}
        onLook={look}
      />

      {owners.value?.problem && list.length === 0 && (
        <Callout tone="danger" title="GitHub couldn't say whose repos there are">
          {owners.value.problem}. Name an owner to look at theirs.
        </Callout>
      )}
      {repos.value?.problem && (
        <Callout tone="danger" title={`GitHub couldn't list ${owner}'s repos`}>
          {repos.value.problem}
          {/SAML|SSO/i.test(repos.value.problem) && (
            <>
              {" "}
              — authorise gh's token for {owner}'s single sign-on, at{" "}
              <a href={`https://github.com/orgs/${owner}/sso`} target="_blank" rel="noreferrer">
                github.com/orgs/{owner}/sso
              </a>
              , then run <code>gh auth refresh</code>.
            </>
          )}
        </Callout>
      )}

      <SettingsAddRepoList
        label={`${owner}'s repos`}
        value={chosen?.name}
        onChange={(name) => setChosen(shown.find((r) => r.name === name))}
        loading={!repos.value ? !!owner : !!looking}
        none={repos.value && !looking ? (q ? "None of these match." : "Nothing to add here.") : undefined}
      >
        {shown.map((r) => (
          <SettingsAddRepoOption key={r.name} repo={r} />
        ))}
      </SettingsAddRepoList>
      {hidden > 0 && <SettingsAddRepoAll hidden={hidden} checked={all} onChange={setAll} />}

      {chosen && <AddRepoAdding key={`${owner}/${chosen.name}`} owner={owner} repo={chosen} taken={taken} groups={groups} groupLabels={groupLabels} onCancel={() => setChosen(undefined)} onAdded={onAdded} />}
    </Dialog>
  );
}
