import { useMemo, useReducer, useState } from "react";
import { useSearchParams } from "react-router";
import { getGroups, getRepos, type Repo, type RepoGroup, shareRepo, useEvents } from "../api";
import { useLoaded } from "../load";
import { Button } from "../ui/Button";
import { Chip } from "../ui/Chip";
import { Count } from "../ui/Count";
import { GroupHead } from "../ui/GroupHead";
import { ErrorCard, Skeleton, toast } from "../ui/kit";
import { AddRepo } from "./AddRepo";
import { Groups, RepoGroupCell } from "./Groups";
import css from "./Repos.module.css";

// Repos is the workspace's repos: the team's, as ws.toml has them, shown
// and not changed here; and yours, in ws.local.toml alone, each of which
// can be given to the team. A repo is added from GitHub (?add=1 opens it,
// as Find's Add a repo does).
export function Repos({ branch, org }: { branch?: string; org?: string }) {
  const { value: repos, error, reload: reloadRepos } = useLoaded<Repo[]>(getRepos, [], { keep: true });
  const { value: groups, reload: reloadGroups } = useLoaded<RepoGroup[]>(getGroups, [], { keep: true });
  const reload = () => Promise.all([reloadRepos(), reloadGroups()]).then(() => undefined);
  const groupNames = useMemo(() => (groups ?? []).map((g) => g.name), [groups]);
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
    <section id="repos" className="settings-block">
      <GroupHead className="settings-group">
        Repos
        {repos && <Count>{repos.length}</Count>}
        <Button size="small" className={css.add} onClick={() => setAdding(true)}>
          Add repo…
        </Button>
      </GroupHead>
      {adding && (
        <AddRepo
          seen={seen}
          groups={groupNames}
          org={org}
          taken={taken}
          onClose={() => setAdding(false)}
          onAdded={() => {
            setAdding(false);
            reload();
          }}
        />
      )}
      <p className="settings-about">
        The team's are in <code>ws.toml</code>: shown here, changed there.
        {yours > 0 && (
          <>
            {" "}
            Yours are in <code>ws.local.toml</code> alone: here, and not in a teammate's workspace.
          </>
        )}
      </p>
      {error && <ErrorCard error={error} retry={reload} />}
      {!repos && !error && <Skeleton rows={3} />}
      {repos && (
        <table className="settings-table">
          <thead>
            <tr>
              <th>Repo</th>
              <th>Called</th>
              <th>On GitHub</th>
              <th>Aliases</th>
              <th>Group</th>
              <th>Lands by</th>
              {yours > 0 && <th aria-label="Actions" />}
            </tr>
          </thead>
          <tbody>
            {repos.map((r) => (
              <tr key={r.name}>
                <td className="mono">
                  <span className={css.name}>
                    <span className={css.key}>{r.name}</span>
                    {r.yours && <Chip title="In ws.local.toml alone">yours</Chip>}
                    {r.cloning && <Chip tone="progress">cloning…</Chip>}
                    {!r.cloned && !r.cloning && !r.hidden && (
                      <Chip tone={r.problem ? "danger" : "quiet"} title={r.problem ? `${r.problem} (ws setup tries again)` : "ws setup clones it"}>
                        not cloned
                      </Chip>
                    )}
                  </span>
                </td>
                <td>
                  <span className="settings-repo">
                    <span className="repo-dot" style={{ background: r.color }} />
                    {r.label}
                  </span>
                </td>
                <td className={`mono dim ${css.github}`}>
                  {r.github ? (
                    <a href={`https://github.com/${r.github}`} target="_blank" rel="noreferrer">
                      {r.github}
                    </a>
                  ) : (
                    "—"
                  )}
                </td>
                <td className="mono dim">{r.aliases?.join(", ") || "—"}</td>
                <td>
                  <RepoGroupCell repo={r} groups={groupNames} done={reload} />
                </td>
                <td className="dim">{r.landing === "direct" ? `straight to ${branch || "its default branch"}` : r.landing ? "pull request" : "—"}</td>
                {yours > 0 && <td className={css.acts}>{r.yours && <Share repo={r.name} done={reload} />}</td>}
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {repos && groups && <Groups groups={groups} repos={repos} done={reload} />}
    </section>
  );
}

// Share gives a repo of yours to the team, asked first: it's a commit on
// the workspace's config repo, pushed, which everyone's next sync brings.
function Share({ repo, done }: { repo: string; done: () => void }) {
  const [asking, setAsking] = useState(false);
  const [busy, setBusy] = useState(false);
  if (!asking)
    return (
      <Button variant="ghost" size="small" onClick={() => setAsking(true)}>
        Share with the team
      </Button>
    );
  const share = () => {
    setBusy(true);
    shareRepo(repo)
      .then((r) => {
        toast(r.message);
        done();
      })
      .catch((e: Error) => toast(e.message, { tone: "danger", ms: 8000 }))
      .finally(() => {
        setBusy(false);
        setAsking(false);
      });
  };
  return (
    <span className={css.ask}>
      <Button variant="primary" size="small" disabled={busy} onClick={share} title={`Adds ${repo} to ws.toml, commits it on the workspace's config repo and pushes it`}>
        {busy ? "Pushing…" : "Commit and push"}
      </Button>
      <Button variant="ghost" size="small" disabled={busy} onClick={() => setAsking(false)}>
        Cancel
      </Button>
    </span>
  );
}
