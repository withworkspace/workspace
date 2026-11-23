import { useEffect, useRef, useState } from "react";
import { Link, useSearchParams } from "react-router";
import { endRun, getRunLog, getRunPast, getRuns, type Names, type RunDetail, type RunsInfo, resyncSilo, type SiloState } from "../api";
import { type Reload, useLoaded } from "../load";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import { toast, useTitle } from "../ui/kit";
import { Skeleton } from "../ui/Skeleton";
import { Empty } from "../ui/Empty";
import { humanError } from "../ui/text";
import { ErrorCard } from "../ui/ErrorCard";
import { Callout } from "../ui/Callout";
import { Copyable } from "../ui/Copyable";
import { Time } from "../ui/time";
import css from "./Runs.module.css";
import { bare, came, commitSaid, forWhom, listed, placeOf, plain, type Said, split } from "./runRows";
import { Chip } from "../ui/Chip";
import { Button } from "../ui/Button";
import { PageHead } from "../ui/PageHead";
import { GroupHead } from "../ui/GroupHead";
import { Summary, SummaryRow } from "../ui/Summary";
import { Row, RowMain, Rows, RowSub } from "../ui/Rows";
import { useWorkspace } from "../workspace/store";

// The Runs page: everything ws is running for someone in this workspace
// (ws run): dev servers, previews, compose stacks, whether you, crew or a
// conversation started them. Each with its link, who it's for (it ends
// when they go), its output, and Stop. Above them, the names it's reached
// by under .localhost (ws router), or how to turn them on.
//
// Those are services. A task is the other type of run: tests, lint, a
// build, which ends by itself. One that's running is with what's
// running; once it has ended it's a result, kept with all it printed,
// the commit it ran at, its reports, and the runs of it before.
// /runs?show=<ref> opens one, as a step's check links to its run.

// lookEvery is how often the page asks again while it's in view: a run
// coming up, or ending, shows within it.
const lookEvery = 3000;

export function RunsPage() {
  useTitle("Runs");
  const { value: info, error, reload } = useLoaded<RunsInfo>(getRuns, [], { keep: true });
  useEffect(() => {
    const t = setInterval(() => {
      if (document.visibilityState === "visible") reload();
    }, lookEvery);
    return () => clearInterval(t);
  }, [reload]);
  const { running, results, ended } = split(info?.runs ?? []);
  const up = new Set(running.map((r) => r.ref));
  const [params] = useSearchParams();
  const show = params.get("show") ?? "";
  return (
    <div className="page">
      <PageHead title="Runs">
        {running.length > 0 && <Chip tone="done">{running.length} running</Chip>}
      </PageHead>
      <p className="lede">
        What ws runs for you and your agents. A service (a dev server, a preview, a compose stack) is given a free port, and ended once whoever it's for goes, or its time's
        up. A task (tests, lint, a build) ends by itself, and what it came to is kept.
      </p>
      {error && !info && <ErrorCard error={error} retry={reload} />}
      {!info && !error && <Skeleton rows={4} />}
      {info && (
        <>
          <NamesPart names={info.names} />
          <section>
            <GroupHead count={running.length}>
              Running
            </GroupHead>
            {running.length === 0 ? (
              <Empty icon="play" title="Nothing's running">
                Crew and conversations start dev servers and run tests with <code>ws run</code>, and so can you: from a terminal, or with Run on a capsule in a mission's Work
                tab. Each shows here, with its link.
              </Empty>
            ) : (
              <Rows className={css.list}>
                {running.map((r) => (
                  <RunRow key={r.ref} r={r} up={up} reload={reload} show={show} />
                ))}
              </Rows>
            )}
          </section>
          {results.length > 0 && (
            <section>
              <GroupHead count={results.length}>Results</GroupHead>
              <p className="quiet-line">What each task last came to, kept with all it printed, its reports, and the runs of it before, while where it ran is there.</p>
              <Rows className={css.list}>
                {results.map((r) => (
                  <RunRow key={r.ref} r={r} up={up} reload={reload} show={show} />
                ))}
              </Rows>
            </section>
          )}
          {info.silos.length > 0 && <SilosPart silos={info.silos} reload={reload} />}
          {ended.length > 0 && (
            <section>
              <GroupHead count={ended.length}>
                Ended
              </GroupHead>
              <p className="quiet-line">Services that ended by themselves, kept an hour with their output.</p>
              <Rows className={css.list}>
                {ended.map((r) => (
                  <RunRow key={r.ref} r={r} up={up} reload={reload} show={show} />
                ))}
              </Rows>
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
    <Summary className={css.names}>
      <SummaryRow label="Names">
        {state}
        {n.name && n.name !== n.want && (
          <div className={cx("dim", css.namesNote)}>
            {n.want} was another workspace's first: this one is {n.name} while that one's daemon runs.
          </div>
        )}
      </SummaryRow>
      <SummaryRow label="Router">
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
      </SummaryRow>
      <SummaryRow label="Settings">
        <Link to="/settings/runs">Local names, in Settings</Link>
      </SummaryRow>
    </Summary>
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
    <RowSub className={cx(css.sub, "run-needs")}>
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
            {n.run && !up.has(n.run) && <Chip tone="waiting">not running</Chip>}
          </span>
        ),
      )}
    </RowSub>
  );
}

// TaskMark is what a task came to, as its row's mark: running, passed,
// failed, or stopped before it ended.
function TaskMark({ r }: { r: RunDetail }) {
  if (r.result === "running")
    return (
      <span className="work-run-mark up" aria-hidden title="Running">
        <span className="work-live-dot" />
      </span>
    );
  const icon = r.result === "passed" ? "check-circle" : r.result === "failed" ? "x-circle" : "circle-dashed";
  return (
    <span className={`work-run-mark task-mark ${r.result}`} role="img" aria-label={r.result} title={came(r)}>
      <Icon name={icon} size={13} />
    </span>
  );
}

// CheckFor is the check of a mission's step a task was run for, going to
// the mission.
function CheckFor({ r }: { r: RunDetail }) {
  const m = useWorkspace((w) => w.missions.find((x) => x.slug === r.check?.mission));
  if (!r.check) return null;
  return (
    <span>
      Check {r.check.n} of step {r.check.step} of <Link to={`/missions/${r.check.mission}`}>{m?.title || r.check.mission}</Link>
    </span>
  );
}

function RunRow({ r, up, reload, show }: { r: RunDetail; up: Set<string>; reload: Reload; show: string }) {
  const conversations = useWorkspace((w) => w.conversations);
  const missions = useWorkspace((w) => w.missions);
  const [busy, setBusy] = useState(false);
  // Asked for by the page's address (?show=): open, and brought into view.
  const shown = show !== "" && (show === r.ref || show.split("#")[0] === r.ref);
  const [open, setOpen] = useState(shown);
  const row = useRef<HTMLLIElement>(null);
  useEffect(() => {
    if (shown) {
      setOpen(true);
      row.current?.scrollIntoView({ block: "center" });
    }
  }, [shown]);
  const live = r.state === "running";
  const task = r.type === "task";
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
  const by = r.by && r.by !== "you" && r.by !== "ws" ? `crew ${r.by}` : r.by;
  return (
    <Row ref={row} className={cx(css.row, !live && !task && css.ended, open && css.open)}>
      <div className={css.line}>
        {task ? (
          <TaskMark r={r} />
        ) : (
          <span className={`work-run-mark${live && r.up ? " up" : ""}`} aria-hidden title={live ? (r.up ? "Up" : "Not up yet") : undefined}>
            {live ? <span className="work-live-dot" /> : <Icon name="circle" size={8} />}
          </span>
        )}
        <RowMain>
          <span className={css.top}>
            <span className="mono work-run-name" title={r.nref ?? r.ref}>
              {r.name}
            </span>
            {task ? (
              <span className={`task-came ${r.result}`}>
                {came(r)}
                {!live && r.ended && (
                  <span className="dim">
                    , <Time iso={r.ended} as="relative" />
                  </span>
                )}
              </span>
            ) : live ? (
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
            {live && !task && !r.up && <Chip tone="quiet">not up</Chip>}
            {task && (r.n ?? 0) > 1 && (
              <span className="dim" title={`${r.past ?? 0} of its earlier runs are kept behind it`}>
                run {r.n}
              </span>
            )}
          </span>
          <RowSub className={css.sub}>
            <span>
              In <To s={placeOf(r, missions)} />
            </span>
            {(live || !task) && (
              <span>
                For <To s={forWhom(r, missions, conversations)} />
              </span>
            )}
            {task && r.commit && <span title={r.commit}>At {commitSaid(r)}</span>}
            <CheckFor r={r} />
            {task && <ReportsSaid r={r} />}
            {live && r.owner !== "run" && (r.neededBy?.length ?? 0) > 0 && <span>Needed by {listed(r.neededBy ?? [])}</span>}
            {live && r.link !== r.local && <span className="mono">{bare(r.local)}</span>}
          </RowSub>
          {live && <NeedsLine r={r} up={up} />}
          {live && r.ends && <span className="work-run-ends">Ends {r.ends}</span>}
        </RowMain>
        <span className={css.when}>{(live || !task) && <Time iso={r.started} as="age" className="age" />}</span>
        <span className={css.actions}>
          <Button variant="ghost" size="small" onClick={() => setOpen(!open)} aria-expanded={open} title={open ? "Hide its output" : "Its output, and how it was started"}>
            <Icon name={open ? "chevron-up" : "terminal"} size={11} /> {open ? "Hide" : "Output"}
          </Button>
          <Button
            variant="ghost"
            size="small"
            disabled={busy}
            onClick={end}
            title={live ? (task ? "Stop it now: it's kept as stopped" : "End it now, and run its recipe's stop") : task ? "Clear it away, with its output, its reports and the runs before it" : "Clear it away, its output with it"}
          >
            {busy ? <Spinner size={11} /> : <Icon name={live ? "stop" : "x"} size={11} />} {live ? "Stop" : "Clear"}
          </Button>
        </span>
      </div>
      {open && (task ? <TaskMore r={r} by={by} show={shown ? show : ""} /> : <RunMore r={r} by={by} />)}
    </Row>
  );
}

// ReportsSaid is the reports kept with a task, by name: one its command
// didn't write says so.
function ReportsSaid({ r }: { r: RunDetail }) {
  if (!r.reports?.length) return null;
  return (
    <span className={css.reports}>
      {r.reports.map((rp) => (
        <Chip
          key={rp.name}
          tone={rp.files > 0 || r.result === "running" ? "quiet" : "waiting"}
          className={css.report}
          title={
            rp.files > 0
              ? `${rp.path}${rp.format ? `, ${rp.format}` : ""}: kept with the run`
              : r.result === "running"
                ? `${rp.path}: kept once it ends`
                : `Its command didn't write ${rp.path}`
          }
        >
          <Icon name="file" size={10} /> {rp.name}
          {rp.files === 0 && r.result !== "running" ? " not written" : ""}
        </Chip>
      ))}
    </span>
  );
}

// TaskMore is a task's run opened: the runs of it that are kept, the
// latest first, one of them picked; that one's facts, and all it printed.
// show, if it names a run of this task (ref#n), picks it.
function TaskMore({ r, by, show }: { r: RunDetail; by?: string; show: string }) {
  const live = r.state === "running";
  const { value } = useLoaded(() => getRunPast(r.ref), [r.ref, r.n, r.state], { keep: true });
  const all = [r, ...(value?.past ?? [])];
  const [picked, setPicked] = useState(show.includes("#") ? show : (r.nref ?? r.ref));
  const one = all.find((x) => x.nref === picked) ?? r;
  return (
    <div className={css.more}>
      {all.length > 1 && (
        <div className={css.past} role="tablist" aria-label={`Runs of ${r.name}`}>
          {all.map((x) => (
            <button
              key={x.nref}
              role="tab"
              aria-selected={x.nref === one.nref}
              className={cx(css.pastRun, x.result === "passed" && css.passed, x.result === "failed" && css.failed, x.nref === one.nref && css.on)}
              onClick={() => setPicked(x.nref ?? x.ref)}
              title={`Run ${x.n}: ${came(x)}${x.commit ? `, at ${commitSaid(x)}` : ""}`}
            >
              <Icon name={x.result === "passed" ? "check-circle" : x.result === "failed" ? "x-circle" : x.result === "running" ? "circle-dot" : "circle-dashed"} size={11} />
              <span>#{x.n}</span>
              <Time iso={x.ended ?? x.started} as="relative" className="dim" />
            </button>
          ))}
        </div>
      )}
      <dl className={css.facts}>
        <dt>Result</dt>
        <dd className={`task-came ${one.result}`}>
          {came(one)}
          {one.ended && (
            <span className="dim">
              , ended <Time iso={one.ended} />
            </span>
          )}
        </dd>
        <dt>Command</dt>
        <dd className="mono">
          <Copyable text={one.command}>{one.command}</Copyable>
        </dd>
        {one.dir && (
          <>
            <dt>Folder</dt>
            <dd className="mono">{one.dir}</dd>
          </>
        )}
        {one.commit && (
          <>
            <dt>Commit</dt>
            <dd>
              <span className="mono">{one.commit.slice(0, 12)}</span>
              {one.dirty && <span className="dim">, with changes that weren't committed</span>}
            </dd>
          </>
        )}
        <dt>Started</dt>
        <dd>
          <Time iso={one.started} />
          {by && <> by {by}</>}
          {one.recipe && (
            <>
              , from its{" "}
              <code>
                [run.{one.type === "task" ? "task" : "service"}.{one.recipe}]
              </code>
            </>
          )}
        </dd>
        {(one.reports?.length ?? 0) > 0 && (
          <>
            <dt>Reports</dt>
            <dd>
              <ReportsSaid r={one} />
            </dd>
          </>
        )}
      </dl>
      <RunOutput ref_={one.nref ?? one.ref} live={live && one.nref === r.nref} />
    </div>
  );
}

// RunOutput is a run's output, asked for again while it runs, kept at
// its end as more comes unless you've scrolled up.
function RunOutput({ ref_, live }: { ref_: string; live: boolean }) {
  // Loaded again as it ends, for its last words.
  const { value, error, reload } = useLoaded(() => getRunLog(ref_), [ref_, live], { keep: true });
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
  if (error && !value) return <div className="quiet-line">Its output can't be read: {humanError(error)}</div>;
  return (
    <pre
      ref={pre}
      className={cx("verify-output", css.log)}
      onScroll={(e) => {
        const el = e.currentTarget;
        stuck.current = el.scrollHeight - el.scrollTop - el.clientHeight < 24;
      }}
    >
      {value ? text || "It printed nothing." : "…"}
    </pre>
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
              , from its{" "}
              <code>
                [run.{r.type === "task" ? "task" : "service"}.{r.recipe}]
              </code>
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
      <GroupHead count={silos.length}>
        Silos
      </GroupHead>
      <p className="quiet-line">
        Each mirrors the capsule it's pointed at, kept by the daemon as files change. <code>ws silo point</code> points one elsewhere.
      </p>
      <Rows className={css.list}>
        {silos.map((st) => (
          <SiloRow key={st.repo} st={st} reload={reload} />
        ))}
      </Rows>
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
    <Row className={cx(css.row, open && css.open)}>
      <div className={css.line}>
        <span className={`work-run-mark${st.watching ? " up" : ""}`} aria-hidden title={st.watching ? "Kept as files change" : "Not kept"}>
          {st.watching ? <span className="work-live-dot" /> : <Icon name="circle" size={8} />}
        </span>
        <RowMain>
          <span className={css.top}>
            <span className="mono work-run-name">{st.repo}</span>
            <span className="dim">on</span>
            <span className="mono">{st.target}</span>
            {trouble && <Chip tone="waiting">{st.error ? "not kept" : last?.error ? "sync failed" : `${hook?.name} failed`}</Chip>}
          </span>
          <RowSub className={css.sub}>
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
          </RowSub>
        </RowMain>
        <span className={css.when} />
        <span className={css.actions}>
          {(hook || last?.error) && (
            <Button variant="ghost" size="small" onClick={() => setOpen(!open)} aria-expanded={open} title={open ? "Hide" : "What its last hook said, and why a sync failed"}>
              <Icon name={open ? "chevron-up" : "terminal"} size={11} /> {open ? "Hide" : "Output"}
            </Button>
          )}
          <Button variant="ghost" size="small" disabled={busy || !!st.error} onClick={resync} title="Give it its capsule's files in full again">
            {busy ? <Spinner size={11} /> : <Icon name="rotate" size={11} />} Resync
          </Button>
        </span>
      </div>
      {open && (
        <div className={css.more}>
          {last?.error && <p className="tone-danger">{last.error}</p>}
          {hook && (
            <>
              <p className="dim">
                Its {hook.name} hook{hook.error ? `: ${hook.error}` : ""}
              </p>
              <pre className={cx("verify-output", css.log)}>{hook.output || "It said nothing."}</pre>
            </>
          )}
        </div>
      )}
    </Row>
  );
}
