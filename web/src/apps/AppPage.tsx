import { useCallback, useEffect, useRef, useState } from "react";
import { useParams } from "react-router";
import { type AppDetail, type AppPointer, getApp, openApp, pointApp, restartApp, stopApp, type Workspace } from "../api";
import { useLatest } from "../load";
import { Icon, type IconName, icons } from "../ui/Icon";
import { ErrorCard, humanError, toast, useTitle } from "../ui/kit";
import { Option, Select } from "../ui/Select";

// An app's page: one of the workspace's own recipes with a page ([run.docs]
// page = … in ws.toml), its run shown in a frame. As the Sketch canvas is
// laid out: a slim bar, and the app as a card under it, inset as the
// sidebar is, with the sidebar folded to a rail beside it (room.ts).
//
// Nothing runs until the page is opened. Opening it starts the run, or
// joins the one that's running, and the page opens it again every
// openEvery while it's in view: that's what keeps it. Once nobody has it
// in view, ws ends it, 15 minutes on (runs.LookLinger). Stopped from here,
// it isn't started again until you say.
//
// The bar points each of its needs at a worktree of the need's repo (the
// ground, the silo, a capsule), which starts the run again on it.

// openEvery is how often an open page says it's still open: well inside
// the 15 minutes a run outlasts nobody looking.
const openEvery = 60_000;
// lookEvery is how often it asks how the run's doing: often while it's
// coming up, less once it's up.
const lookStarting = 1500;
const lookUp = 10_000;

export function AppPage({ workspace }: { workspace: Workspace }) {
  const { name = "" } = useParams();
  const entry = workspace.apps?.find((a) => a.name === name);
  useTitle(entry?.title ?? name);
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

  // Opened as the page is, and again every openEvery while it's in view
  // and you haven't stopped it.
  const stoppedRef = useRef(stopped);
  stoppedRef.current = stopped;
  useEffect(() => {
    setApp(undefined);
    setError(undefined);
    setStopped(false);
    stoppedRef.current = false;
    void take(() => openApp(name));
    const again = () => {
      if (document.visibilityState === "visible" && !stoppedRef.current) void take(() => openApp(name), true);
    };
    const t = setInterval(again, openEvery);
    document.addEventListener("visibilitychange", again);
    return () => {
      clearInterval(t);
      document.removeEventListener("visibilitychange", again);
    };
  }, [name, take]);

  // How the run's doing: asked again while it's in view.
  const ready = !!app?.ready;
  useEffect(() => {
    const t = setInterval(
      () => {
        if (document.visibilityState === "visible") void take(() => getApp(name), true);
      },
      ready ? lookUp : lookStarting,
    );
    return () => clearInterval(t);
  }, [name, ready, take]);

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
    await act("stop", () => stopApp(name));
  };
  const start = async () => {
    setStopped(false);
    await act("start", () => openApp(name));
  };

  if (!entry) {
    return (
      <div className="page">
        <ErrorCard title="No such app" error={`The workspace has no app called ${name}: an app is a recipe in ws.toml with a page ([run.${name}] page = { title = "…" }).`} />
      </div>
    );
  }
  const run = app?.run;
  const running = run?.state === "running";
  const icon: IconName = entry.icon && entry.icon in icons ? (entry.icon as IconName) : "box";
  return (
    <div className="app-view">
      <header className="app-bar">
        <Icon name={icon} className="app-icon" />
        <h1 className="app-title">{entry.title}</h1>
        {app?.pointers.map((p) => (
          <span key={p.need} className="app-point" title={pointTitle(p)}>
            <span className="app-point-need">{p.need}</span>
            <Select value={p.at || "ground"} onChange={(v) => void point(p, v)} disabled={busy !== undefined} aria-label={`What ${p.need} is pointed at`}>
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
            <button type="button" className="ghost icon-button" title="Start it again" aria-label="Start it again" disabled={busy !== undefined} onClick={() => void act("restart", () => restartApp(name))}>
              <Icon name="rotate" size={15} />
            </button>
            <button type="button" className="ghost icon-button" title="Stop it" aria-label="Stop it" disabled={busy !== undefined} onClick={() => void stop()}>
              <Icon name="stop" size={15} />
            </button>
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
            <ErrorCard title={`Couldn't start ${entry.title}`} error={error} retry={() => void start()} />
          </div>
        ) : ready && run ? (
          // Started again, it's another run: the frame loads afresh.
          <iframe key={`${run.pid}`} src={run.link} title={entry.title} />
        ) : (
          <Waiting app={app} title={entry.title} stopped={stopped} />
        )}
      </div>
    </div>
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
        <Icon name="spinner" size={13} className="spin" /> Starting
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
