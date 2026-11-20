import { useEffect, useRef, useState } from "react";
import { Link } from "react-router";
import { endRun, getRunLog, getRuns, type Names, type RunDetail, type RunsInfo, resyncSilo, type SiloState, type Workspace } from "../api";
import { type Reload, useLoaded } from "../load";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Callout, Copyable, Empty, ErrorCard, humanError, Skeleton, toast, useTitle } from "../ui/kit";
import { Time } from "../ui/time";
import css from "./Runs.module.css";
import { bare, forWhom, listed, placeOf, plain, type Said, split } from "./runRows";
import { Chip } from "../ui/Chip";
import { Count } from "../ui/Count";

// The Runs page: everything ws is running for someone in this workspace
// (ws run): dev servers, previews, compose stacks, whether you, crew or a
// conversation started them. Each with its link, who it's for (it ends
// when they go), its output, and Stop. Above them, the names it's reached
// by under .localhost (ws router), or how to turn them on.

// lookEvery is how often the page asks again while it's in view: a run
// coming up, or ending, shows within it.
const lookEvery = 3000;

export function RunsPage({ workspace }: { workspace: Workspace }) {
  useTitle("Runs");
  const { value: info, error, reload } = useLoaded<RunsInfo>(getRuns, [], { keep: true });
  useEffect(() => {
    const t = setInterval(() => {
      if (document.visibilityState === "visible") reload();
    }, lookEvery);
    return () => clearInterval(t);
  }, [reload]);
  const { running, ended } = split(info?.runs ?? []);
  const up = new Set(running.map((r) => r.ref));
  return (
    <div className="page">
      <header className="page-head">
        <h1>Runs</h1>
        {running.length > 0 && <Chip tone="done">{running.length} running</Chip>}
      </header>
      <p className="lede">
        What ws runs for you and your agents: dev servers, previews, compose stacks. Each is given a free port, and ends by itself once whoever it's for goes, or its time's
        up.
      </p>
      {error && !info && <ErrorCard error={error} retry={reload} />}
      {!info && !error && <Skeleton rows={4} />}
      {info && (
        <>
          <NamesPart names={info.names} />
          <section>
            <h2 className="group">
              Running <Count>{running.length}</Count>
            </h2>
            {running.length === 0 ? (
              <Empty icon="play" title="Nothing's running">
                Crew and conversations start dev servers with <code>ws run</code>, and so can you: from a terminal, or with Run on a capsule in a mission's Work tab. Each shows
                here, with its link.
              </Empty>
            ) : (
              <ul className={cx("rows", css.list)}>
                {running.map((r) => (
                  <RunRow key={r.ref} r={r} up={up} workspace={workspace} reload={reload} />
                ))}
              </ul>
            )}
          </section>
          {info.silos.length > 0 && <SilosPart silos={info.silos} reload={reload} />}
          {ended.length > 0 && (
            <section>
              <h2 className="group">
                Ended <Count>{ended.length}</Count>
              </h2>
              <p className="quiet-line">Runs that ended by themselves, kept an hour with their output.</p>
              <ul className={cx("rows", css.list)}>
                {ended.map((r) => (
                  <RunRow key={r.ref} r={r} up={up} workspace={workspace} reload={reload} />
                ))}
              </ul>
            </section>
          )}
        </>
      )}
    </div>
  );
}

// NamesPart is where the workspace's names stand, or how to have them.
function NamesPart({ names: n }: { names: Names }) {
  const example = `web.<capsule>.<repo>.${n.want}.localhost`;
  if (!n.on) {
    return (
      <Callout tone="quiet" icon="link" title="Runs are reached at 127.0.0.1 and a port">
        Turn on <Link to="/settings/runs">local names</Link> and each gets a name that stays put, <code>{example}</code>, with this page at <code>{n.want}.localhost</code>.
        Browsers send any <code>.localhost</code> to this machine by themselves.
      </Callout>
    );
  }
  let state: React.ReactNode;
  if (!n.port) state = <>No router is serving yet: this workspace's daemon becomes it within a few seconds.</>;
  else if (!n.name) state = <>The daemon hasn't written this workspace's routes yet: it does within a few seconds.</>;
  else
    state = (
      <>
        <a href={n.web} target="_blank" rel="noopener noreferrer" className="mono">
          {bare(n.web ?? "")}
          <Icon name="arrow-up-right" size={11} />
        </a>
        <span className="dim"> and each run below it, as {example.replace(`.${n.want}.`, `.${n.name}.`)}</span>
      </>
    );
  return (
    <dl className={cx("summary settings", css.names)}>
      <div className="summary-row">
        <dt>Names</dt>
        <dd>
          {state}
          {n.name && n.name !== n.want && (
            <div className={cx("dim", css.namesNote)}>
              {n.want} was another workspace's first: this one is {n.name} while that one's daemon runs.
            </div>
          )}
        </dd>
      </div>
      <div className="summary-row">
        <dt>Router</dt>
        <dd>
          {n.port ? (
            <>
              On :{n.port}
              {n.fallback && <span className="dim">, as 80 is another program's or kept by the system</span>}
              {n.heldBy ? <span className="dim">, held by the daemon at pid {n.heldBy}</span> : null}
              <span className="dim">. One serves every workspace on this machine.</span>
            </>
          ) : (
            <span className="dim">None running</span>
          )}
        </dd>
      </div>
      <div className="summary-row">
        <dt>Settings</dt>
        <dd>
          <Link to="/settings/runs">Local names, in Settings</Link>
        </dd>
      </div>
    </dl>
  );
}

function To({ s }: { s: Said }) {
  return s.to ? <Link to={s.to}>{s.text}</Link> : <>{s.text}</>;
}

// NeedsLine is what a run needs: each need's name, the run that meets it
// (or the address outside ws that does), and the path of the run's own
// name it answers at, if it's mounted. up is the refs of what's running:
// a need whose run has ended says so.
function NeedsLine({ r, up }: { r: RunDetail; up: Set<string> }) {
  if (!r.needs?.length) return null;
  return (
    <span className={cx("row-sub", css.sub, "run-needs")}>
      {r.needs.map((n) =>
        n.folder ? (
          <span key={n.name}>
            Needs {n.name}:{" "}
            <Link to={n.worktree === "ground" ? `/files/${n.repo}` : `/files/${n.repo}/${n.worktree}`} className="mono" title={n.env ? `Handed to it as $${n.env}: ${n.dir}` : n.dir}>
              {n.repo} / {n.worktree}
            </Link>
          </span>
        ) : (
          <span key={n.name}>
            Needs {n.name}:{" "}
            <a href={n.address} target="_blank" rel="noopener noreferrer" className="mono" title={n.env ? `Handed to it as $${n.env}: ${n.address}` : n.address}>
              {n.run ?? bare(n.address)}
            </a>
            {n.mount && r.link !== r.local && (
              <>
                , at{" "}
                <a href={r.link.replace(/\/$/, "") + n.mount} target="_blank" rel="noopener noreferrer" className="mono" title={`${n.name} answers below this run's own name`}>
                  {n.mount}
                </a>
              </>
            )}
            {n.run && !up.has(n.run) && <span className="chip tone-waiting">not running</span>}
          </span>
        ),
      )}
    </span>
  );
}

function RunRow({ r, up, workspace, reload }: { r: RunDetail; up: Set<string>; workspace: Workspace; reload: Reload }) {
  const [busy, setBusy] = useState(false);
  const [open, setOpen] = useState(false);
  const live = r.state === "running";
  const end = () => {
    setBusy(true);
    endRun(r.ref)
      .then(() => {
        toast(live ? `Stopped ${r.name}` : `Cleared ${r.name}`, { tone: "done" });
        return reload();
      })
      .catch((e: Error) => toast(humanError(e.message), { tone: "danger" }))
      .finally(() => setBusy(false));
  };
  const by = r.by && r.by !== "you" ? `crew ${r.by}` : r.by;
  return (
    <li className={cx("row", css.row, !live && css.ended, open && css.open)}>
      <div className={css.line}>
        <span className={`work-run-mark${live && r.up ? " up" : ""}`} aria-hidden title={live ? (r.up ? "Up" : "Not up yet") : undefined}>
          {live ? <span className="work-live-dot" /> : <Icon name="circle" size={8} />}
        </span>
        <span className="row-main">
          <span className={css.top}>
            <span className="mono work-run-name" title={r.ref}>
              {r.name}
            </span>
            {live ? (
              <a
                href={r.link}
                target="_blank"
                rel="noopener noreferrer"
                className="mono work-run-link"
                title={`${r.up ? "Open it" : "Nothing's listening on its port yet"}${r.link !== r.local ? `: also ${bare(r.local)}` : ""}`}
              >
                {bare(r.link)}
                <Icon name="arrow-up-right" size={11} />
              </a>
            ) : (
              <span className="dim">
                {r.state === "exited" ? `exited ${r.status ?? 0}` : "gone: ended outside ws"}
                {r.ended && (
                  <>
                    , <Time iso={r.ended} as="relative" />
                  </>
                )}
              </span>
            )}
            {live && !r.up && <Chip tone="quiet">not up</Chip>}
          </span>
          <span className={cx("row-sub", css.sub)}>
            <span>
              In <To s={placeOf(r, workspace.missions)} />
            </span>
            <span>
              For <To s={forWhom(r, workspace.missions, workspace.conversations)} />
            </span>
            {live && r.owner !== "run" && (r.neededBy?.length ?? 0) > 0 && <span>Needed by {listed(r.neededBy ?? [])}</span>}
            {live && r.link !== r.local && <span className="mono">{bare(r.local)}</span>}
          </span>
          {live && <NeedsLine r={r} up={up} />}
          {live && r.ends && <span className="work-run-ends">Ends {r.ends}</span>}
        </span>
        <span className={css.when}>
          <Time iso={r.started} as="age" className="age" />
        </span>
        <span className={css.actions}>
          <button className="ghost small" onClick={() => setOpen(!open)} aria-expanded={open} title={open ? "Hide its output" : "Its output, and how it was started"}>
            <Icon name={open ? "chevron-up" : "terminal"} size={11} /> {open ? "Hide" : "Output"}
          </button>
          <button className="ghost small" disabled={busy} onClick={end} title={live ? "End it now, and run its recipe's stop" : "Clear it away, its output with it"}>
            <Icon name={busy ? "spinner" : live ? "stop" : "x"} size={11} /> {live ? "Stop" : "Clear"}
          </button>
        </span>
      </div>
      {open && <RunMore r={r} by={by} />}
    </li>
  );
}

// RunMore is a run's output, asked for again while it runs, and how it
// was started.
function RunMore({ r, by }: { r: RunDetail; by?: string }) {
  const live = r.state === "running";
  const { value, error, reload } = useLoaded(() => getRunLog(r.ref), [r.ref], { keep: true });
  const pre = useRef<HTMLPreElement>(null);
  const stuck = useRef(true); // at the bottom: new output keeps it there
  useEffect(() => {
    if (!live) return;
    const t = setInterval(() => {
      if (document.visibilityState === "visible") reload();
    }, 2000);
    return () => clearInterval(t);
  }, [live, reload]);
  const text = value ? plain(value.log) : "";
  useEffect(() => {
    const el = pre.current;
    if (el && stuck.current) el.scrollTop = el.scrollHeight;
  }, [text]);
  return (
    <div className={css.more}>
      <dl className={css.facts}>
        <dt>Command</dt>
        <dd className="mono">
          <Copyable text={r.command}>{r.command}</Copyable>
        </dd>
        {r.dir && (
          <>
            <dt>Folder</dt>
            <dd className="mono">{r.dir}</dd>
          </>
        )}
        <dt>Started</dt>
        <dd>
          <Time iso={r.started} />
          {by && <> by {by}</>}
          {r.recipe && (
            <>
              , from its <code>[run.{r.recipe}]</code>
            </>
          )}
        </dd>
        {live && (
          <>
            <dt>Process</dt>
            <dd>
              pid {r.pid}, on port {r.port}
              {r.link !== r.local && (
                <>
                  {" "}
                  (<a href={r.local} target="_blank" rel="noopener noreferrer" className="mono">
                    {bare(r.local)}
                  </a>
                  )
                </>
              )}
            </dd>
          </>
        )}
      </dl>
      {error && !value ? (
        <div className="quiet-line">Its output can't be read: {humanError(error)}</div>
      ) : (
        <pre
          ref={pre}
          className={cx("verify-output", css.log)}
          onScroll={(e) => {
            const el = e.currentTarget;
            stuck.current = el.scrollHeight - el.scrollTop - el.clientHeight < 24;
          }}
        >
          {value ? text || "Nothing yet." : "…"}
        </pre>
      )}
    </div>
  );
}

// SilosPart is the repos' silos, as the daemon keeping them has them:
// what each is pointed at, its last sync, and its last hook.
function SilosPart({ silos, reload }: { silos: SiloState[]; reload: Reload }) {
  return (
    <section>
      <h2 className="group">
        Silos <span className="count">{silos.length}</span>
      </h2>
      <p className="quiet-line">
        Each mirrors the capsule it's pointed at, kept by the daemon as files change. <code>ws silo point</code> points one elsewhere.
      </p>
      <ul className="rows run-rows">
        {silos.map((st) => (
          <SiloRow key={st.repo} st={st} reload={reload} />
        ))}
      </ul>
    </section>
  );
}

function SiloRow({ st, reload }: { st: SiloState; reload: Reload }) {
  const [busy, setBusy] = useState(false);
  const [open, setOpen] = useState(false);
  const last = st.syncs?.[st.syncs.length - 1];
  const hook = st.hook;
  const trouble = st.error || last?.error || hook?.error;
  const resync = () => {
    setBusy(true);
    resyncSilo(st.repo)
      .then(() => {
        toast(`Resynced ${st.repo}'s silo`, { tone: "done" });
        return reload();
      })
      .catch((e: Error) => toast(humanError(e.message), { tone: "danger" }))
      .finally(() => setBusy(false));
  };
  return (
    <li className={`row run-row${open ? " open" : ""}`}>
      <div className="run-row-line">
        <span className={`work-run-mark${st.watching ? " up" : ""}`} aria-hidden title={st.watching ? "Kept as files change" : "Not kept"}>
          {st.watching ? <span className="work-live-dot" /> : <Icon name="circle" size={8} />}
        </span>
        <span className="row-main">
          <span className="run-row-top">
            <span className="mono work-run-name">{st.repo}</span>
            <span className="dim">on</span>
            <span className="mono">{st.target}</span>
            {trouble && <span className="chip tone-waiting">{st.error ? "not kept" : last?.error ? "sync failed" : `${hook?.name} failed`}</span>}
          </span>
          <span className="row-sub run-row-sub">
            {st.error ? (
              <span>{st.error}</span>
            ) : (
              <span>
                Watching {st.dirs} folder{st.dirs === 1 ? "" : "s"}
              </span>
            )}
            {last && (
              <span>
                {last.error ? "Last sync failed" : `Last synced ${last.files} file${last.files === 1 ? "" : "s"}`}
                {last.full && last.why ? ` (${last.why})` : ""}, <Time iso={last.at} as="relative" />
              </span>
            )}
            {hook && (
              <span>
                {hook.name} {hook.error ? "failed" : "ran"} <Time iso={hook.at} as="relative" />
              </span>
            )}
          </span>
        </span>
        <span className="run-row-when" />
        <span className="run-row-actions">
          {(hook || last?.error) && (
            <button className="ghost small" onClick={() => setOpen(!open)} aria-expanded={open} title={open ? "Hide" : "What its last hook said, and why a sync failed"}>
              <Icon name={open ? "chevron-up" : "terminal"} size={11} /> {open ? "Hide" : "Output"}
            </button>
          )}
          <button className="ghost small" disabled={busy || !!st.error} onClick={resync} title="Give it its capsule's files in full again">
            <Icon name={busy ? "spinner" : "rotate"} size={11} /> Resync
          </button>
        </span>
      </div>
      {open && (
        <div className="run-more">
          {last?.error && <p className="tone-danger">{last.error}</p>}
          {hook && (
            <>
              <p className="dim">
                Its {hook.name} hook{hook.error ? `: ${hook.error}` : ""}
              </p>
              <pre className="verify-output run-log">{hook.output || "It said nothing."}</pre>
            </>
          )}
        </div>
      )}
    </li>
  );
}
