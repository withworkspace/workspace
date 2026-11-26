import { useMemo, useReducer } from "react";
import { useSearchParams } from "react-router";
import { getGroups, getRepos, type Repo, type RepoGroup, useEvents } from "../api";
import { useLoaded } from "../load";
import { ErrorCard } from "../ui/ErrorCard";
import { Skeleton } from "../ui/Skeleton";
import { AddRepo } from "./AddRepo";
import { Groups } from "./Groups";
import { RepoGroupCell } from "./RepoGroupCell";
import { RepoShare } from "./RepoShare";
import { SettingsBlock } from "./SettingsBlock";
import { SettingsRepoRow } from "./SettingsRepoRow";
import { SettingsReposTable } from "./SettingsReposTable";
import { SettingsReposTitle } from "./SettingsReposTitle";

// Repos is the workspace's repos: the team's, as ws.toml has them, shown
// and not changed here; and yours, in ws.local.toml alone, each of which
// can be given to the team. A repo is added from GitHub (?add=1 opens it,
// as Find's Add a repo does).
export function Repos({ branch, org }: { branch?: string; org?: string }) {
  const { value: repos, error, reload: reloadRepos } = useLoaded<Repo[]>(getRepos, [], { keep: true });
  const { value: groups, reload: reloadGroups } = useLoaded<RepoGroup[]>(getGroups, [], { keep: true });
  const reload = () => Promise.all([reloadRepos(), reloadGroups()]).then(() => undefined);
  const groupNames = useMemo(() => (groups ?? []).map((g) => g.name), [groups]);
  const groupLabels = useMemo(() => new Map((groups ?? []).map((g) => [g.name, g.label || g.name])), [groups]);
  // A repo added is cloned behind, and GitHub's asked behind what the
  // picker shows: the github topic says when either's done (one stream,
  // for the table and the picker both).
  const [seen, bump] = useReducer((n: number) => n + 1, 0);
  const again = () => {
    reload();
    bump();
  };
  // On hello too: a look that ended before the stream opened is caught up
  // with then.
  useEvents("github", { hello: again, changed: again });
  const [params, setParams] = useSearchParams();
  const adding = params.has("add");
  const setAdding = (on: boolean) =>
    setParams(
      (p) => {
        const next = new URLSearchParams(p);
        if (on) next.set("add", "1");
        else next.delete("add");
        return next;
      },
      { replace: true },
    );
  const taken = useMemo(() => new Set((repos ?? []).flatMap((r) => [r.name, ...(r.aliases ?? [])])), [repos]);
  const yours = repos?.filter((r) => r.yours).length ?? 0;
  return (
    <SettingsBlock
      id="repos"
      title={<SettingsReposTitle count={repos?.length} onAdd={() => setAdding(true)} />}
      about={
        <>
          The team's are in <code>ws.toml</code>: edit them there.
          {yours > 0 && (
            <>
              {" "}Yours are in <code>ws.local.toml</code>: only in your workspace.
            </>
          )}
        </>
      }
    >
      {adding && (
        <AddRepo
          seen={seen}
          groups={groupNames}
          groupLabels={groupLabels}
          org={org}
          taken={taken}
          onClose={() => setAdding(false)}
          onAdded={() => {
            setAdding(false);
            reload();
          }}
        />
      )}
      {error && <ErrorCard error={error} retry={reload} />}
      {!repos && !error && <Skeleton rows={3} />}
      {repos && (
        <SettingsReposTable acts={yours > 0}>
          {repos.map((r) => (
            <SettingsRepoRow
              key={r.name}
              repo={r}
              lands={r.landing === "direct" ? `straight to ${branch || "its default branch"}` : r.landing ? "pull request" : "—"}
              group={<RepoGroupCell repo={r} groups={groupNames} labels={groupLabels} done={reload} />}
              acts={yours > 0}
            >
              {r.yours && <RepoShare repo={r.name} done={reload} />}
            </SettingsRepoRow>
          ))}
        </SettingsReposTable>
      )}
      {repos && groups && <Groups groups={groups} repos={repos} done={reload} />}
    </SettingsBlock>
  );
}
