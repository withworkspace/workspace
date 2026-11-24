import { useCallback, useEffect, useRef, useState } from "react";
import { useParams } from "react-router";
import { type AppAt, type AppDetail, type AppPointer, getApp, openApp, pointApp, type RecipeRender, restartApp, stopApp, useEventsOf } from "../api";
import { useLatest } from "../load";
import { Icon, type IconName, icons } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import { useTitle } from "../ui/useTitle";
import { toast } from "../ui/toast";
import { humanError } from "../ui/text";
import { ErrorCard } from "../ui/ErrorCard";
import { Option, Select } from "../ui/Select";
import { Button, buttonClass } from "../ui/Button";
import { useWorkspace } from "../workspace/store";

// An app: a recipe with a page, its run shown in a frame. As the Sketch
// canvas is laid out: a slim bar, and the app as a card under it, inset as
// the sidebar is. One of the workspace's own (ws.toml) has a page of its
// own, with the sidebar folded to a rail beside it (room.ts); a repo's
// (ws.repo.toml) is a tab of each of its worktrees, on the repo's page
// (AppFrame, with at).
//
// Nothing runs until the page is opened. Opening it starts the run, or
// joins the one that's running, and the page follows it over the tab's
// socket while it's in view (app:<name>, app:<name>@<repo>/<worktree>):
// that's what keeps it, and the page's told as it starts, comes up or
// ends. Once nobody has it in view, ws ends it, 15 minutes on
// (runs.LookLinger). Ended by itself, it's opened again (once in openEvery
// at most, for one that keeps failing); stopped from here, it isn't
// started again until you say.
//
// One whose recipe says render = "tab" isn't framed: the card says it's
// opened in a tab of its own, and opens it, once it's up. The page still
// keeps it while it's in view, as the framed one does.
//
// A workspace app's bar points each of its needs at a worktree of the
// need's repo (the ground, the silo, a capsule), which starts the run
// again on it. A worktree's app's needs are the nearest to it, as any
// run's are: its bar says what met each.

// openEvery is how often a run that's ended by itself is opened again, at
// most.
const openEvery = 60_000;

export function AppPage() {
  const workspace = useWorkspace();
  const { name = "" } = useParams();
  const entry = workspace.apps?.find((a) => a.name === name);
  useTitle(entry?.title ?? name);
  if (!entry) {
    return (
      <div className="page">
        <ErrorCard title="No such app" error={`The workspace has no app called ${name}: an app is a service in ws.toml with a page ([run.service.${name}] page = { title = "…" }).`} />
      </div>
    );
  }
  return (
    <div className="app-view">
      <AppFrame name={name} title={entry.title} icon={entry.icon} render={entry.render} />
    </div>
  );
}

// AppFrame is an app's bar and card: the workspace's (no at), or one of a
// worktree's (at), whose tab already names it, so its bar doesn't. render
// is how its recipe says it's shown: framed, or in a tab of its own.
export function AppFrame({ name, title, icon: iconName, at, render }: { name: string; title: string; icon?: string; at?: AppAt; render?: RecipeRender }) {
  const [app, setApp] = useState<AppDetail>();
  const [error, setError] = useState<string>();
  // stopped: you stopped it here, so the page doesn't start it again.
  const [stopped, setStopped] = useState(false);
  const [busy, setBusy] = useState<string>();
  const latest = useLatest();

  const take = useCallback(
    async (load: () => Promise<AppDetail>, quiet = false) => {
      const fresh = latest();
      try {
        const d = await load();
        if (!fresh()) return;
        setApp(d);
        setError(undefined);
        return d;
      } catch (e) {
        if (!fresh()) return;
        if (!quiet) setError(humanError(String(e)));
      }
    },
    [latest],
  );

  // Opened as the page is, and again as it's shown again, unless you've
  // stopped it.
  const stoppedRef = useRef(stopped);
  stoppedRef.current = stopped;
  const opened = useRef(0);
  useEffect(() => {
    setApp(undefined);
    setError(undefined);
    setStopped(false);
    stoppedRef.current = false;
    opened.current = Date.now();
    void take(() => openApp(name, at));
    const again = () => {
      if (document.visibilityState !== "visible" || stoppedRef.current) return;
      opened.current = Date.now();
      void take(() => openApp(name, at), true);
    };
    document.addEventListener("visibilitychange", again);
    return () => document.removeEventListener("visibilitychange", again);
    // at is a new object each render: its repo and worktree are what matter.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [name, at?.repo, at?.worktree, take]);

  const ready = !!app?.ready;
  // How the run's doing: asked when it's told it's moved. One that's ended
  // by itself is opened again, but not more than once in openEvery.
  const topic = at ? `app:${name}@${at.repo}/${at.worktree}` : `app:${name}`;
  useEventsOf([topic], {
    changed: () =>
      void take(() => getApp(name, at), true).then((d) => {
        const ended = d && d.run?.state !== "running";
        if (ended && !stoppedRef.current && Date.now() - opened.current > openEvery) {
          opened.current = Date.now();
          void take(() => openApp(name, at), true);
        }
      }),
  });

  const act = async (what: string, load: () => Promise<AppDetail>) => {
    setBusy(what);
    const d = await take(load);
    setBusy(undefined);
    return d;
  };
  const point = async (p: AppPointer, at: string) => {
    const d = await act(`point:${p.need}`, () => pointApp(name, p.need, at === "ground" ? "" : at));
    if (d?.said) toast(d.said);
  };
  const stop = async () => {
    setStopped(true);
    await act("stop", () => stopApp(name, at));
  };
  const start = async () => {
    setStopped(false);
    await act("start", () => openApp(name, at));
  };

  const run = app?.run;
  const running = run?.state === "running";
  const icon: IconName = iconName && iconName in icons ? (iconName as IconName) : "box";
  return (
    <>
      <header className="app-bar">
        {!at && (
          <>
            <Icon name={icon} className="app-icon" />
            <h1 className="app-title">{title}</h1>
          </>
        )}
        {at && run?.needs?.map((n) => (
          <span key={n.name} className="app-need" title={n.folder ? `${n.name}: ${n.dir}` : `${n.name}: ${n.address}`}>
            <span className="app-point-need">{n.name}</span> <span className="dim">from</span> {n.folder ? (n.repo === n.name ? n.worktree : `${n.repo} / ${n.worktree}`) : (n.run ?? n.address)}
          </span>
        ))}
        {app?.pointers.map((p) => (
          <span key={p.need} className="app-point" title={pointTitle(p)}>
            <span className="app-point-need">{p.need}</span>
            <Select size="small" value={p.at || "ground"} onChange={(v) => void point(p, v)} disabled={busy !== undefined} aria-label={`What ${p.need} is pointed at`}>
              {p.choices.map((c) => (
                <Option key={c} value={c} note={c === "ground" ? "nearest" : undefined}>
                  {c}
                </Option>
              ))}
            </Select>
          </span>
        ))}
        <span className="app-spacer" />
        <AppState app={app} stopped={stopped} />
        {running ? (
          <>
            <Button variant="ghost" square type="button" title="Start it again" aria-label="Start it again" disabled={busy !== undefined} onClick={() => void act("restart", () => restartApp(name, at))}>
              <Icon name="rotate" size={15} />
            </Button>
            <Button variant="ghost" square type="button" title="Stop it" aria-label="Stop it" disabled={busy !== undefined} onClick={() => void stop()}>
              <Icon name="stop" size={15} />
            </Button>
            {ready && (
              <a className="app-link" href={run.link} target="_blank" rel="noreferrer" title={`Open ${run.link} in a tab of its own`} aria-label="Open in a tab of its own">
                <Icon name="external" size={15} />
              </a>
            )}
          </>
        ) : (
          app && (
            <button type="button" className="button" disabled={busy !== undefined} onClick={() => void start()}>
              <Icon name="play" size={14} /> Start
            </button>
          )
        )}
      </header>
      <div className="app-card">
        {error && !app ? (
          <div className="app-waiting">
            <ErrorCard title={`Couldn't start ${title}`} error={error} retry={() => void start()} />
          </div>
        ) : ready && run && render === "tab" ? (
          <div className="app-waiting">
            <p>{title} opens in a tab of its own, as its recipe says. It's kept running while you have this open, and 15 minutes after.</p>
            <a className={buttonClass({ variant: "primary" })} href={run.link} target="_blank" rel="noreferrer" title={run.link}>
              <Icon name="external" size={14} /> Open {title}
            </a>
          </div>
        ) : ready && run ? (
          // Started again, it's another run: the frame loads afresh.
          <iframe key={`${run.pid}`} src={run.link} title={title} />
        ) : (
          <Waiting app={app} title={title} stopped={stopped} />
        )}
      </div>
    </>
  );
}

// pointTitle says what a need is, on its pointer.
function pointTitle(p: AppPointer): string {
  return p.folder ? `${p.need}: ${p.repo}'s folder, from the worktree picked` : `${p.need}: ${p.repo}'s ${p.recipe}, run in the worktree picked`;
}

// AppState is how the run stands, in a word, with when it ends on hover.
function AppState({ app, stopped }: { app?: AppDetail; stopped: boolean }) {
  const run = app?.run;
  if (!app) return null;
  if (run?.state === "running") {
    return app.ready ? (
      <span className="app-state tone-done" title={`It ends ${run.ends}.`}>
        <span className="app-dot" /> Up
      </span>
    ) : (
      <span className="app-state" title={`It ends ${run.ends}.`}>
        <Spinner size={13} /> Starting
      </span>
    );
  }
  if (run && run.state === "exited") {
    return (
      <span className="app-state tone-danger">
        <Icon name="x-circle" size={13} /> Exited {run.status}
      </span>
    );
  }
  return <span className="app-state dim">{stopped ? "Stopped" : "Not running"}</span>;
}

// Waiting is the card while there's nothing to frame: the run coming up,
// with what it's saying; or why there isn't one.
function Waiting({ app, title, stopped }: { app?: AppDetail; title: string; stopped: boolean }) {
  const run = app?.run;
  let said: string;
  if (!app) said = `Starting ${title}…`;
  else if (run?.state === "running") said = `Starting ${title}: it's shown here once it answers.`;
  else if (run?.state === "exited") said = `${title} ended by itself (exit status ${run.status}). Its last words are below.`;
  else if (stopped) said = `${title} is stopped. Start it to see it again.`;
  else said = `${title} isn't running. Start it to see it.`;
  return (
    <div className="app-waiting">
      <p>{said}</p>
      {app?.log && <pre className="app-log">{app.log}</pre>}
    </div>
  );
}
