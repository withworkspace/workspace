import { type ReactNode, useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router";
import { type AppAt, getPlaceRuns, lookedAtPlace, openApp, type PlaceRunsInfo, type Workspace } from "../api";
import { AppFrame } from "../apps/AppPage";
import { type BrowserProps, FileBrowser } from "./Explorer";
import { type Reload, useLoaded } from "../load";
import { RunRow } from "../runs/Runs";
import { split } from "../runs/runRows";
import { Icon, type IconName, icons } from "../ui/Icon";
import { Empty, ErrorCard, humanError, toast } from "../ui/kit";

// A worktree's tabs, on its repo's page: its Files, its Runs, and each of
// its apps (a recipe of its ws.repo.toml with a page), its run framed. The
// tab is the address's ?view= (files, unsaid), so a link opens a
// capsule's docs. While the page is open it says so every lookedEvery,
// which keeps what was started from it: its apps, and what its Runs ran.

const lookedEvery = 60_000;
// How often its runs are asked after: often on Runs, where they're shown.
const runsOften = 3000;
const runsSeldom = 15_000;

export interface WorktreeTabs {
  tabs: ReactNode;
  // other is what's shown in place of the tree and the file, on a tab
  // that isn't Files.
  other?: ReactNode;
}

export function useWorktreeTabs(at: AppAt, workspace: Workspace): WorktreeTabs {
  const [params] = useSearchParams();
  const view = params.get("view") ?? "files";
  const { value: info, error, reload } = useLoaded<PlaceRunsInfo>(() => getPlaceRuns(at), [at.repo, at.worktree]);

  useEffect(() => {
    const t = setInterval(
      () => {
        if (document.visibilityState === "visible") reload();
      },
      view === "runs" ? runsOften : runsSeldom,
    );
    return () => clearInterval(t);
  }, [reload, view]);
  useEffect(() => {
    const look = () => {
      if (document.visibilityState === "visible") lookedAtPlace(at).catch(() => {});
    };
    const t = setInterval(look, lookedEvery);
    return () => clearInterval(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [at.repo, at.worktree]);

  const to = (v: string) => {
    const next = new URLSearchParams(params);
    if (v === "files") next.delete("view");
    else next.set("view", v);
    const q = next.toString();
    return { search: q ? `?${q}` : "" };
  };
  const running = info ? split(info.runs).running.length : 0;
  const tab = (v: string, label: string, icon: IconName, count?: number) => (
    <Link key={v} to={to(v)} className="worktree-tab" aria-current={view === v ? "page" : undefined} replace>
      <Icon name={icon} size={14} />
      {label}
      {!!count && <span className="count">{count}</span>}
    </Link>
  );
  const tabs = (
    <nav className="worktree-tabs" aria-label="What's here">
      {tab("files", "Files", "file")}
      {tab("runs", "Runs", "play", running)}
      {info?.apps.map((a) => tab(a.name, a.title, a.icon && a.icon in icons ? (a.icon as IconName) : "box"))}
    </nav>
  );

  let other: ReactNode;
  if (view === "runs") {
    other = error && !info ? <ErrorCard title="Couldn't read its runs" error={error} retry={reload} /> : <RunsHere at={at} info={info} reload={reload} workspace={workspace} />;
  } else if (view !== "files") {
    const app = info?.apps.find((a) => a.name === view);
    if (app) other = <AppFrame key={`${at.repo}/${at.worktree}/${app.name}`} name={app.name} title={app.title} icon={app.icon} at={at} />;
    else if (info)
      other = (
        <div className="page narrow">
          <Empty icon="box" title={`No ${view} here`}>
            This worktree has no app called {view}: an app is a recipe in its ws.repo.toml with a page.
          </Empty>
        </div>
      );
  }
  return { tabs, other };
}

// RunsHere is a worktree's Runs tab: what's running in it, and its
// recipes, each started from here (lasting while the page is open, as an
// app does) or, for an app, opened in its tab.
function RunsHere({ at, info, reload, workspace }: { at: AppAt; info?: PlaceRunsInfo; reload: Reload; workspace: Workspace }) {
  const [busy, setBusy] = useState<string>();
  if (!info) return null;
  const { running, ended } = split(info.runs);
  const up = new Set(running.map((r) => r.ref));
  const start = async (name: string) => {
    setBusy(name);
    try {
      await openApp(name, at);
      await reload();
    } catch (e) {
      toast(`Couldn't start ${name}: ${humanError(String(e))}`, { tone: "danger" });
    }
    setBusy(undefined);
  };
  return (
    <div className="page worktree-runs">
      <section>
        <h2 className="group">
          Running <span className="count">{running.length}</span>
        </h2>
        {running.length === 0 ? (
          <p className="quiet-line">Nothing's running here. A recipe below starts one, for as long as this page is open; so does ws run, from a terminal here.</p>
        ) : (
          <ul className="rows run-rows">
            {running.map((r) => (
              <RunRow key={r.ref} r={r} up={up} workspace={workspace} reload={reload} />
            ))}
          </ul>
        )}
      </section>
      <section>
        <h2 className="group">
          Recipes <span className="count">{info.recipes.length}</span>
        </h2>
        {info.recipes.length === 0 ? (
          <p className="quiet-line">
            No recipes: a repo says how it's run as <code>[run.&lt;name&gt;]</code> in its ws.repo.toml.
          </p>
        ) : (
          <ul className="rows recipe-rows">
            {info.recipes.map((rc) => {
              const on = info.runs.find((r) => r.recipe === rc.name && r.state === "running");
              return (
                <li key={rc.name} className="row recipe-row">
                  <span className="recipe-name">{rc.name}</span>
                  <code className="recipe-command dim">{rc.command}</code>
                  <span className="recipe-act">
                    {rc.app ? (
                      <Link to={{ search: `?view=${encodeURIComponent(rc.name)}` }} className="button-link" replace>
                        Open {rc.app.title}
                      </Link>
                    ) : on ? (
                      <span className="chip tone-done">running</span>
                    ) : (
                      <button type="button" className="small" disabled={busy !== undefined} onClick={() => void start(rc.name)}>
                        <Icon name="play" size={12} /> Run
                      </button>
                    )}
                  </span>
                </li>
              );
            })}
          </ul>
        )}
      </section>
      {ended.length > 0 && (
        <section>
          <h2 className="group">
            Ended <span className="count">{ended.length}</span>
          </h2>
          <ul className="rows run-rows">
            {ended.map((r) => (
              <RunRow key={r.ref} r={r} up={up} workspace={workspace} reload={reload} />
            ))}
          </ul>
        </section>
      )}
    </div>
  );
}

// TabbedBrowser is a worktree's files with its tabs: the repo's page, and a
// mission's capsule's.
export function TabbedBrowser({ at, workspace, ...props }: BrowserProps & { at: AppAt; workspace: Workspace }) {
  const { tabs, other } = useWorktreeTabs(at, workspace);
  return <FileBrowser {...props} tabs={tabs} other={other} />;
}
