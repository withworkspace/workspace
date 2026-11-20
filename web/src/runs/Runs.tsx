import { useEffect, useRef, useState } from "react";
import { Link } from "react-router";
import { endRun, getRunLog, getRuns, type Names, type RunDetail, type RunsInfo, type Workspace } from "../api";
import { type Reload, useLoaded } from "../load";
import { Icon } from "../ui/Icon";
import { Callout, Copyable, Empty, ErrorCard, humanError, Skeleton, toast, useTitle } from "../ui/kit";
import { Time } from "../ui/time";
import { bare, forWhom, placeOf, plain, type Said, split } from "./runRows";

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
  return (
    <div className="page runs-page">
      <header className="page-head">
        <h1>Runs</h1>
        {running.length > 0 && <span className="chip tone-done">{running.length} running</span>}
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
              Running <span className="count">{running.length}</span>
            </h2>
            {running.length === 0 ? (
              <Empty icon="play" title="Nothing's running">
                Crew and conversations start dev servers with <code>ws run</code>, and so can you: from a terminal, or with Run on a capsule in a mission's Work tab. Each shows
                here, with its link.
              </Empty>
            ) : (
              <ul className="rows run-rows">
                {running.map((r) => (
                  <RunRow key={r.ref} r={r} workspace={workspace} reload={reload} />
                ))}
              </ul>
            )}
          </section>
          {ended.length > 0 && (
            <section>
              <h2 className="group">
                Ended <span className="count">{ended.length}</span>
              </h2>
              <p className="quiet-line">Runs that ended by themselves, kept an hour with their output.</p>
              <ul className="rows run-rows">
                {ended.map((r) => (
                  <RunRow key={r.ref} r={r} workspace={workspace} reload={reload} />
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
    <dl className="summary settings runs-names">
      <div className="summary-row">
        <dt>Names</dt>
        <dd>
          {state}
          {n.name && n.name !== n.want && (
            <div className="dim runs-names-note">
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

function RunRow({ r, workspace, reload }: { r: RunDetail; workspace: Workspace; reload: Reload }) {
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
    <li className={`row run-row${live ? "" : " ended"}${open ? " open" : ""}`}>
      <div className="run-row-line">
        <span className={`work-run-mark${live && r.up ? " up" : ""}`} aria-hidden title={live ? (r.up ? "Up" : "Not up yet") : undefined}>
          {live ? <span className="work-live-dot" /> : <Icon name="circle" size={8} />}
        </span>
        <span className="row-main">
          <span className="run-row-top">
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
            {live && !r.up && <span className="chip tone-quiet">not up</span>}
          </span>
          <span className="row-sub run-row-sub">
            <span>
              In <To s={placeOf(r, workspace.missions)} />
            </span>
            <span>
              For <To s={forWhom(r, workspace.missions, workspace.conversations)} />
            </span>
            {live && r.link !== r.local && <span className="mono">{bare(r.local)}</span>}
          </span>
          {live && r.ends && <span className="work-run-ends">Ends {r.ends}</span>}
        </span>
        <span className="run-row-when">
          <Time iso={r.started} as="age" className="age" />
        </span>
        <span className="run-row-actions">
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
    <div className="run-more">
      <dl className="run-facts">
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
          className="verify-output run-log"
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
