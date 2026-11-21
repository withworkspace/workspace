import { useState } from "react";
import { getRepos, type Repo, shareRepo } from "../api";
import { useLoaded } from "../load";
import { Button } from "../ui/Button";
import { Chip } from "../ui/Chip";
import { Count } from "../ui/Count";
import { GroupHead } from "../ui/GroupHead";
import { ErrorCard, Skeleton, toast } from "../ui/kit";
import css from "./Repos.module.css";

// Repos is the workspace's repos: the team's, as ws.toml has them, shown
// and not changed here; and yours, in ws.local.toml alone, each of which
// can be given to the team.
export function Repos({ branch }: { branch?: string }) {
  const { value: repos, error, reload } = useLoaded<Repo[]>(getRepos, [], { keep: true });
  const yours = repos?.filter((r) => r.yours).length ?? 0;
  return (
    <section id="repos" className="settings-block">
      <GroupHead className="settings-group">
        Repos
        {repos && <Count>{repos.length}</Count>}
      </GroupHead>
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
                    {r.name}
                    {r.yours && <Chip title="In ws.local.toml alone">yours</Chip>}
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
                <td className="dim">{r.group || "—"}</td>
                <td className="dim">{r.landing === "direct" ? `straight to ${branch || "its default branch"}` : r.landing ? "pull request" : "—"}</td>
                {yours > 0 && <td className={css.acts}>{r.yours && <Share repo={r.name} done={reload} />}</td>}
              </tr>
            ))}
          </tbody>
        </table>
      )}
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
