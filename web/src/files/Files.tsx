import { switchKeys } from "./keys";
import { useSaid } from "../keymap/keymap";
import { useState } from "react";
import { Crumbs } from "../ui/Crumbs";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { docRef, GROUND, getRepos, type Repo, worktreeAPI } from "../api";
import { useMedia } from "../ui/useMedia";
import { useTitle } from "../ui/useTitle";
import { Loading } from "../ui/Loading";
import { Empty } from "../ui/Empty";
import { ErrorCard } from "../ui/ErrorCard";
import { Keys } from "../ui/Keys";
import { TabbedBrowser } from "./WorktreeTabs";
import { useLoaded } from "../load";
import { useWorkspace } from "../workspace/store";
import { filesBarCrumbsClass } from "./Browser/FilesBar";
import { FilesMission } from "./Repos/FilesMission";
import { FilesPick } from "./Repos/FilesPick";
import { FilesReposPage } from "./Repos/FilesReposPage";
import { RepoColumn } from "./Repos/RepoColumn";
import { WorktreeSwitch } from "./Repos/WorktreeSwitch";

// Files: every repo in the workspace, as ws labels them (its display name
// and colour, by group), and the files of its ground (its default branch,
// as it's checked out) or any of its capsules, read as a mission's
// explorer reads a capsule's. The title says which, and switches (w).

export function FilesPage() {
  const { repo = "", capsule } = useParams();
  const [params, setParams] = useSearchParams();
  const navigate = useNavigate();
  const narrow = useMedia("(max-width: 860px)");
  const { value: repos, error, reload: load } = useLoaded<Repo[]>(getRepos, [], { keep: true });
  const [switching, setSwitching] = useState(false);
  const switchKey = useSaid(switchKeys.open);
  useTitle(repo ? undefined : "Repos");

  const r = repos?.find((x) => x.name === repo);
  // A capsule a mission has: it's reviewed there, and read again as it moves.
  const mission = useWorkspace((w) => (capsule ? w.missions.find((m) => m.capsules.some((c) => c.repo === repo && c.name === capsule)) : undefined));
  const readsSeen = useWorkspace((w) => w.readsSeen);
  const column = repos && <RepoColumn repos={repos} current={repo} />;

  if (error && !repos) {
    return (
      <div className="page narrow">
        <ErrorCard title="Could not load repos" error={error} retry={load} />
      </div>
    );
  }
  if (!repos) return <Loading what="Loading repos…" />;
  if (!repo || !r) {
    return (
      <FilesReposPage narrow={narrow} column={column}>
        {!narrow && (
          <FilesPick>
            {repo && !r ? (
              <Empty icon="box" title={`No repo ${repo}`}>
                No repo by that name. Pick one from the list.
              </Empty>
            ) : (
              <Empty icon="layers" title="Pick a repo">
                Browse files on the default branch (ground) or any capsule. Switch with the title (
                <Keys k={switchKey ?? "w"} />).
              </Empty>
            )}
          </FilesPick>
        )}
      </FilesReposPage>
    );
  }
  const name = capsule ?? GROUND;
  const api = worktreeAPI(repo, name);
  return (
    <>
      <TabbedBrowser
        key={api}
        at={{ repo, worktree: name }}
        api={api}
        label={`${r.label}/${capsule ?? "ground"}`}
        before={narrow ? undefined : column}
        updated={`${mission?.updated ?? ""}:${readsSeen ?? ""}`}
        reviewTo={mission ? `/missions/${mission.slug}/review/${repo}/${capsule}` : undefined}
        docOf={(path) => docRef(repo, name, path)}
        search={capsule ? { repo, worktree: capsule } : undefined}
        onEscape={() => {
          if (switching) return;
          const next = new URLSearchParams(params);
          next.delete("path");
          if (params.get("path")) setParams(next);
          else navigate("/files");
        }}
        empty={capsule ? "The capsule is empty." : "The repo is empty."}
        aside={
          mission && <FilesMission to={`/missions/${mission.slug}`} title={mission.title} />
        }
        head={
          <>
            {narrow && (
              <Crumbs className={filesBarCrumbsClass} steps={[{ to: "/files", label: "Repos" }]} />
            )}
            <WorktreeSwitch repo={r} current={name} open={switching} setOpen={setSwitching} />
          </>
        }
      />
    </>
  );
}
