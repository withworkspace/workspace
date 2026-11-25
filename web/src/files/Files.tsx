import { useEffect, useMemo, useRef, useState } from "react";
import { Crumbs } from "../ui/Crumbs";
import { Link, useLocation, useNavigate, useParams, useSearchParams } from "react-router";
import { docRef, GROUND, getRepos, getWorktrees, type Repo, type Worktree, worktreeAPI } from "../api";
import { useKeys } from "../keys";
import { Icon } from "../ui/Icon";
import { chipClass } from "../ui/Chip";
import { useMedia } from "../ui/useMedia";
import { useTitle } from "../ui/useTitle";
import { Loading } from "../ui/Loading";
import { Empty } from "../ui/Empty";
import { ErrorCard } from "../ui/ErrorCard";
import { Keys } from "../ui/Keys";
import { TabbedBrowser } from "./WorktreeTabs";
import { useLoaded } from "../load";
import { Count } from "../ui/Count";
import { GroupHead, groupHeadClass } from "../ui/GroupHead";
import { useWorkspace } from "../workspace/store";

// Files: every repo in the workspace, as ws labels them (its display name
// and colour, by group), and the files of its ground (its default branch,
// as it's checked out) or any of its capsules, read as a mission's
// explorer reads a capsule's. The title says which, and switches (w).

export const filesKeys: [string, string][] = [
  ["w", "Switch to the ground, or another capsule"],
  ["t / mod+P", "Find a file"],
  ["mod+shift+F", "Search what a capsule's files say (on a repo's ground: Find, tagged to it)"],
  ["j / k", "Next / previous in the tree"],
  ["⏎ / o", "Open a file, or open or close a folder"],
  ["⏎", "On the markdown file that's open: open it as a document, to comment on"],
  ["h / l", "Close / open a folder"],
  ["c", "Only what's changed, or every file (a capsule)"],
  ["[ / ]", "Previous / next changed file"],
  ["m", "Markdown: rendered, or as text"],
  ["v", "Review the capsule's changes (a mission's)"],
  ["y", "Copy the file's path"],
  ["p", "Panes: the repos, the tree and the file; the tree and the file; or the file alone"],
  ["r", "Reload"],
  ["esc", "Close the file"],
];

export function FilesPage() {
  const { repo = "", capsule } = useParams();
  const [params, setParams] = useSearchParams();
  const navigate = useNavigate();
  const narrow = useMedia("(max-width: 860px)");
  const { value: repos, error, reload: load } = useLoaded<Repo[]>(getRepos, [], { keep: true });
  const [switching, setSwitching] = useState(false);
  useTitle(repo ? undefined : "Repos");

  const r = repos?.find((x) => x.name === repo);
  // A capsule a mission has: it's reviewed there, and read again as it moves.
  const mission = useWorkspace((w) => (capsule ? w.missions.find((m) => m.capsules.some((c) => c.repo === repo && c.name === capsule)) : undefined));
  const readsSeen = useWorkspace((w) => w.readsSeen);
  const column = repos && <RepoColumn repos={repos} current={repo} />;

  if (error && !repos) {
    return (
      <div className="page narrow">
        <ErrorCard title="Couldn't list the repos" error={error} retry={load} />
      </div>
    );
  }
  if (!repos) return <Loading what="Listing the repos…" />;
  if (!repo || !r) {
    return (
      <div className={`files-page${narrow ? " narrow" : ""}`}>
        {column}
        {!narrow && (
          <div className="files-pick">
            {repo && !r ? (
              <Empty icon="box" title={`No repo ${repo}`}>
                The workspace has no repo called that. Pick one of its repos.
              </Empty>
            ) : (
              <Empty icon="layers" title="Pick a repo">
                Its files as they are on its default branch (the ground), or in any of its capsules: its title switches between them (<Keys k="w" />).
              </Empty>
            )}
          </div>
        )}
      </div>
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
          mission && (
            <Link to={`/missions/${mission.slug}`} className={chipClass(undefined, "wt-mission")} title={`The mission this capsule is for: ${mission.title}`}>
              <Icon name="flag" size={12} />
              <span>{mission.title}</span>
            </Link>
          )
        }
        head={
          <>
            {narrow && (
              <Crumbs className="explorer-crumbs" steps={[{ to: "/files", label: "Repos" }]} />
            )}
            <WorktreeSwitch repo={r} current={name} open={switching} setOpen={setSwitching} />
          </>
        }
      />
    </>
  );
}

// RepoColumn is the workspace's repos, as ws labels them: ungrouped first,
// then each group in its order (ws.toml's order, then by name) under its
// display name, hidden groups after, folded (unless what's open is in one). A
// repo that's a space of documents has Documents for its home: it's folded
// away at the foot, In Documents.
function RepoColumn({ repos, current }: { repos: Repo[]; current: string }) {
  const spaces = repos.filter((r) => r.documents);
  const groups = useMemo(() => {
    const by = new Map<string, Repo[]>();
    for (const r of repos) if (!r.documents) by.set(r.group ?? "", [...(by.get(r.group ?? "") ?? []), r]);
    return [...by].sort(([a, ra], [b, rb]) => (a === "" ? -1 : b === "" ? 1 : Number(!!ra[0].hidden) - Number(!!rb[0].hidden) || (ra[0].groupOrder ?? 0) - (rb[0].groupOrder ?? 0) || a.localeCompare(b)));
  }, [repos]);
  const row = (r: Repo) => (
    <Link key={r.name} to={`/files/${encodeURIComponent(r.name)}`} className={`repo-row${r.name === current ? " on" : ""}${r.cloned ? "" : " uncloned"}`} title={r.cloned ? undefined : `Not cloned (ws setup ${r.name})`} aria-current={r.name === current ? "page" : undefined}>
      <span className="repo-dot" style={{ background: r.color }} />
      <span className="repo-label">{r.label}</span>
      {r.capsules.length > 0 && (
        <span className="repo-capsules" title={`${r.capsules.length} ${r.capsules.length === 1 ? "capsule" : "capsules"}`}>
          {r.capsules.length}
        </span>
      )}
    </Link>
  );
  return (
    <nav className="files-repos" aria-label="Repos">
      <div className="files-repos-head">
        <h1>Repos</h1>
        <span className="dim">{repos.length}</span>
      </div>
      <div className="files-repos-list">
        {groups.map(([g, rs]) =>
          rs[0].hidden ? (
            <details key={g} className="repo-group" open={rs.some((r) => r.name === current)}>
              <summary className={groupHeadClass("repo-group-head")}>
                {rs[0].groupLabel || g} <span className="dim">· hidden</span> <Count>{rs.length}</Count>
              </summary>
              {rs.map(row)}
            </details>
          ) : (
            <section key={g || "-"} className="repo-group">
              {g && (
                <GroupHead className="repo-group-head" count={rs.length}>
                  {rs[0].groupLabel || g}
                </GroupHead>
              )}
              {rs.map(row)}
            </section>
          ),
        )}
        {spaces.length > 0 && (
          <details className="repo-group in-documents" open={spaces.some((r) => r.name === current)}>
            <summary className={groupHeadClass("repo-group-head")} title="Repos of documents: read in Documents, by their titles. Their files are here too.">
              In Documents <Count>{spaces.length}</Count>
              <Link to="/documents" className="group-link" title="Open Documents" onClick={(e) => e.stopPropagation()}>
                Open
              </Link>
            </summary>
            {spaces.map(row)}
          </details>
        )}
      </div>
    </nav>
  );
}

// WorktreeSwitch is the title: the repo and which of its worktrees this
// is, the ground or a capsule, and (open) the others, to switch to. The
// file open stays open, if it's in the other.
function WorktreeSwitch({
  repo,
  current,
  open,
  setOpen,
}: {
  repo: Repo;
  current: string;
  open: boolean;
  setOpen: (v: boolean) => void;
}) {
  const capsulesSeen = useWorkspace((w) => w.capsulesSeen);
  const missions = useWorkspace((w) => w.missions);
  const navigate = useNavigate();
  const location = useLocation();
  // Answered at once, with what git hasn't said yet unread: read again when
  // it has (the workspace's capsulesSeen moves).
  const { value: wts } = useLoaded<Worktree[]>(() => getWorktrees(repo.name).catch(() => []), [repo.name, capsulesSeen], { keep: true });
  // Until their branches are read, what the repo's listing says.
  const list: Worktree[] = wts ?? [{ name: GROUND, ground: true, branch: "" }, ...repo.capsules.map((c) => ({ name: c, branch: "" }))];
  const [sel, setSel] = useState(0);
  useEffect(() => {
    if (open) setSel(Math.max(0, list.findIndex((w) => w.name === current)));
    // Only as it opens.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);
  const go = (w: Worktree) => {
    setOpen(false);
    if (w.name !== current) navigate(`/files/${encodeURIComponent(repo.name)}${w.ground ? "" : `/${encodeURIComponent(w.name)}`}${location.search}`);
  };
  const box = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!open) return;
    const away = (e: MouseEvent) => !box.current?.contains(e.target as Node) && setOpen(false);
    document.addEventListener("mousedown", away);
    return () => document.removeEventListener("mousedown", away);
  }, [open, setOpen]);
  useKeys(open ? {} : { w: () => setOpen(true) });
  useKeys(
    open
      ? {
          Escape: () => setOpen(false),
          ArrowDown: () => setSel((i) => Math.min(i + 1, list.length - 1)),
          j: () => setSel((i) => Math.min(i + 1, list.length - 1)),
          ArrowUp: () => setSel((i) => Math.max(i - 1, 0)),
          k: () => setSel((i) => Math.max(i - 1, 0)),
          Enter: () => list[sel] && go(list[sel]),
          w: () => setOpen(false),
        }
      : {},
    { modal: open },
  );
  const here = list.find((w) => w.name === current);
  const missionOf = (name: string) => missions.find((m) => m.capsules.some((c) => c.repo === repo.name && c.name === name))?.title;
  return (
    <div className="wt-switch" ref={box}>
      <button className="wt-title" aria-haspopup="listbox" aria-expanded={open} onClick={() => setOpen(!open)} title={`${current === GROUND ? "The default branch, as it's checked out" : "A capsule"}. Switch to the ground or another capsule (w)`}>
        <span className="repo-dot" style={{ background: repo.color }} />
        <span className="wt-repo">{repo.label}</span>
        <span className="wt-sep">/</span>
        <span className={`wt-name${current === GROUND ? " ground" : ""}`}>{current === GROUND ? "ground" : current}</span>
        {here?.dirty && current !== GROUND && <span className="dirty-dot" title="Changes not committed" />}
        <Icon name="chevron-down" size={13} className="wt-chevron" />
      </button>
      {open && (
        <ul className="wt-menu" role="listbox" aria-label={`${repo.label}'s ground and capsules`}>
          {list.map((w, i) => {
            const m = missionOf(w.name);
            return (
              <li key={w.name} role="option" aria-selected={w.name === current} className={`${i === sel ? "on" : ""}${w.name === current ? " current" : ""}`} onMouseMove={() => i !== sel && setSel(i)} onClick={() => go(w)}>
                <Icon name={w.ground ? "layers" : "box"} size={14} className="dim" />
                <span className="wt-item">
                  <span className="wt-item-name">{w.ground ? "ground" : w.name}</span>
                  <span className="wt-item-sub">
                    {w.ground ? (w.branch ? `${w.branch}, the default branch` : "the default branch") : m ? m : w.branch && w.branch !== w.name ? w.branch : "a capsule"}
                  </span>
                </span>
                {!!w.ahead && <span className="wt-ab" title={`${w.ahead} ahead of its upstream`}>↑{w.ahead}</span>}
                {!!w.behind && <span className="wt-ab" title={`${w.behind} behind its upstream`}>↓{w.behind}</span>}
                {w.dirty && <span className="dirty-dot" title="Changes not committed" />}
                {w.name === current && <Icon name="check" size={13} className="wt-check" />}
              </li>
            );
          })}
          {wts && wts.length <= 1 && <li className="wt-none dim">No capsules yet: ws lift {repo.name} &lt;name&gt; makes one.</li>}
        </ul>
      )}
    </div>
  );
}
