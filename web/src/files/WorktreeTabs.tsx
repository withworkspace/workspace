import { pageSocket } from "../socket";
import { type ReactNode, useEffect } from "react";
import { useSearchParams } from "react-router";
import { type AppAt, getWorktreeRuns, type WorktreeRunsInfo } from "../api";
import { AppFrame } from "../apps/AppFrame";
import { useLoaded } from "../load";
import { type IconName, icons } from "../ui/Icon";
import { Empty } from "../ui/Empty";
import { ErrorCard } from "../ui/ErrorCard";
import { type BrowserProps, FileBrowser } from "./Browser/FileBrowser";
import { FilesTab, FilesTabs } from "./Browser/FilesTabs";
import { WorktreeRuns } from "./Runs";

// A worktree's tabs, on its repo's page: its Files, its Runs, and each of
// its apps (a recipe of its ws.repo.toml with a page), its run framed. The
// tab is the address's ?view= (files, unsaid), so a link opens a
// capsule's docs.
//
// Its runs are followed over the tab's socket (worktree:<repo>/<name>),
// read again when a run starts, comes up or ends; shown or hidden, on
// every tab, as following it is what says the page is open, which keeps
// what was started from it, from its Runs or as one of its apps.

export interface WorktreeTabs {
  tabs: ReactNode;
  // other is what's shown in place of the tree and the file, on a tab
  // that isn't Files.
  other?: ReactNode;
}

export function useWorktreeTabs(at: AppAt): WorktreeTabs {
  const [params] = useSearchParams();
  const view = params.get("view") ?? "files";
  const { value: info, error, reload } = useLoaded<WorktreeRunsInfo>(() => getWorktreeRuns(at.repo, at.worktree), [at.repo, at.worktree]);
  useEffect(() => pageSocket.follow(`worktree:${at.repo}/${at.worktree}`, { following: reload, changed: () => reload() }), [at.repo, at.worktree, reload]);

  const to = (v: string) => {
    const next = new URLSearchParams(params);
    if (v === "files") next.delete("view");
    else next.set("view", v);
    const q = next.toString();
    return { search: q ? `?${q}` : "" };
  };
  const running = info?.runs.filter((r) => r.state === "running").length ?? 0;
  const tab = (v: string, label: string, icon: IconName, count?: number) => <FilesTab key={v} to={to(v)} icon={icon} label={label} count={count} current={view === v} />;
  const tabs = (
    <FilesTabs>
      {tab("files", "Files", "file")}
      {tab("runs", "Runs", "play", running)}
      {info?.apps.map((a) => tab(a.name, a.title, a.icon && a.icon in icons ? (a.icon as IconName) : "box"))}
    </FilesTabs>
  );

  let other: ReactNode;
  if (view === "runs") {
    other =
      error && !info ? (
        <ErrorCard title="Could not load runs" error={error} retry={reload} />
      ) : (
        <WorktreeRuns repo={at.repo} name={at.worktree} value={info} reload={reload} />
      );
  } else if (view !== "files") {
    const app = info?.apps.find((a) => a.name === view);
    if (app) other = <AppFrame key={`${at.repo}/${at.worktree}/${app.name}`} name={app.name} title={app.title} icon={app.icon} render={app.render} at={at} />;
    else if (info)
      other = (
        <div className="page narrow">
          <Empty icon="box" title={`No ${view} here`}>
            No app called {view} here. Apps are recipes in ws.repo.toml with a page.
          </Empty>
        </div>
      );
  }
  return { tabs, other };
}

// TabbedBrowser is a worktree's files with its tabs: the repo's page, and a
// mission's capsule's.
export function TabbedBrowser({ at, ...props }: BrowserProps & { at: AppAt }) {
  const { tabs, other } = useWorktreeTabs(at);
  return <FileBrowser {...props} tabs={tabs} other={other} />;
}
