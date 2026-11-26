import { useCallback, useEffect, useRef, useState } from "react";
import { type AppAt, type AppDetail, type AppPointer, getApp, openApp, pointApp, type RecipeRender, restartApp, stopApp, useEventsOf } from "../api";
import { useLatest } from "../load";
import { Button, buttonClass } from "../ui/Button";
import { ErrorCard } from "../ui/ErrorCard";
import { Icon, type IconName, icons } from "../ui/Icon";
import { Option, Select } from "../ui/Select";
import { humanError } from "../ui/strings";
import { toast } from "../ui/toast";
import { AppBar, AppBarLink, AppBarNeed, AppBarPointer } from "./AppBar";
import { AppCard } from "./AppCard";
import { AppState } from "./AppState";
import { AppWaiting } from "./AppWaiting";
import { pointTitle, standingOf, waitingWords } from "./words";

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
  const standing = standingOf(app, stopped);
  const end = (
    <>
      {standing && <AppState {...standing} />}
      {running ? (
        <>
          <Button variant="ghost" square type="button" title="Start again" aria-label="Start again" disabled={busy !== undefined} onClick={() => void act("restart", () => restartApp(name, at))}>
            <Icon name="rotate" size={15} />
          </Button>
          <Button variant="ghost" square type="button" title="Stop" aria-label="Stop" disabled={busy !== undefined} onClick={() => void stop()}>
            <Icon name="stop" size={15} />
          </Button>
          {ready && <AppBarLink href={run.link} />}
        </>
      ) : (
        app && (
          <Button type="button" disabled={busy !== undefined} onClick={() => void start()}>
            <Icon name="play" size={14} /> Start
          </Button>
        )
      )}
    </>
  );
  return (
    <>
      <AppBar icon={at ? undefined : icon} title={at ? undefined : title} end={end}>
        {at &&
          run?.needs?.map((n) => (
            <AppBarNeed key={n.name} name={n.name} title={n.folder ? `${n.name}: ${n.dir}` : `${n.name}: ${n.address}`} from={n.folder ? (n.repo === n.name ? n.worktree : `${n.repo} / ${n.worktree}`) : (n.run ?? n.address)} />
          ))}
        {app?.pointers.map((p) => (
          <AppBarPointer key={p.need} name={p.need} title={pointTitle(p)}>
            <Select size="small" value={p.at || "ground"} onChange={(v) => void point(p, v)} disabled={busy !== undefined} aria-label={`Capsule for ${p.need}`}>
              {p.choices.map((c) => (
                <Option key={c} value={c} note={c === "ground" ? "nearest" : undefined}>
                  {c}
                </Option>
              ))}
            </Select>
          </AppBarPointer>
        ))}
      </AppBar>
      <AppCard>
        {error && !app ? (
          <AppWaiting>
            <ErrorCard title={`Could not start ${title}`} error={error} retry={() => void start()} />
          </AppWaiting>
        ) : ready && run && render === "tab" ? (
          <AppWaiting>
            <p>{title} opens in new tab. Kept running while you have this open.</p>
            <a className={buttonClass({ variant: "primary" })} href={run.link} target="_blank" rel="noreferrer" title={run.link}>
              <Icon name="external" size={14} /> Open {title}
            </a>
          </AppWaiting>
        ) : ready && run ? (
          // Started again, it's another run: the frame loads afresh.
          <iframe key={`${run.pid}`} src={run.link} title={title} />
        ) : (
          <AppWaiting log={app?.log}>
            <p>{waitingWords(app, title, stopped)}</p>
          </AppWaiting>
        )}
      </AppCard>
    </>
  );
}
