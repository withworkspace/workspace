import { useMemo } from "react";
import { Link, useParams } from "react-router";
import { getOrchestrator, getOrchestratorRun, type OrchestratorInfo, type OrchestratorRun, type OrchestratorRunDetail, type Workspace } from "../api";
import { Markdown } from "../markdown/LazyMarkdown";
import { Calls } from "../ui/Calls";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/inline";
import { Callout, Copyable, Empty, ErrorCard, Skeleton, useTitle } from "../ui/kit";
import { headlineStatus, Status, type StatusInfo } from "../ui/status";
import { duration, Time } from "../ui/time";
import { useLoaded } from "../load";

// The orchestrator's page: whether it's on and how it's set up, the
// missions it's on now, and its latest runs; each run's own page has what
// it was told, what it said and every call it made, refused ones marked.

// changedKey is when anything about the missions last changed: pages here
// load again when it moves (a run's end is logged on its mission).
function changedKey(workspace: Workspace) {
  return workspace.missions.map((m) => m.updated).join("|");
}

// lookedAt is what a run looked at, its trigger said once: "c1 (done)".
export const lookedAt = (r: Pick<OrchestratorRun, "for" | "trigger">) =>
  !r.trigger || r.for.includes(`(${r.trigger})`) ? r.for : `${r.for} (${r.trigger})`;

export function OrchestratorPage({ workspace }: { workspace: Workspace }) {
  useTitle("Orchestrator");
  const key = useMemo(() => changedKey(workspace), [workspace]);
  const { value: info, error, reload: load } = useLoaded<OrchestratorInfo>(getOrchestrator, [key], { keep: true });
  const deciding = workspace.missions.filter((m) => m.headline.kind === "deciding");
  const on: StatusInfo | undefined = info && (info.problem ? { icon: "x-circle", tone: "danger", words: "Can't run" } : info.enabled ? { icon: "check-circle", tone: "done", words: "On" } : { icon: "pause", tone: "quiet", words: "Off" });
  return (
    <div className="page">
      <header className="page-head">
        <h1>Orchestrator</h1>
        {on && <Status s={on} className="mission-state" />}
      </header>
      <p className="lede">
        An agent that decides what happens between crew: when crew report done or go quiet, when you answer it or review, when a PR changes. It reads the mission, then
        accepts a stage, dispatches the next crew, or asks you.
      </p>
      {error && <ErrorCard error={error} retry={load} />}
      {!info && !error && <Skeleton rows={5} />}
      {info && (
        <>
          {info.problem && (
            <Callout tone="danger" title="It can't run as it's set up">
              <Inline text={info.problem} />
            </Callout>
          )}
          {!info.enabled && (
            <Callout tone="quiet" icon="pause" title="It's off: nothing moves missions on but you">
              Turn it on for you in <Link to="/settings/missions#orchestrator">Settings</Link>, or for everyone in <code>ws.toml</code>:
              <div className="command snippet">
                <code>{"[orchestrator]\nenabled = true"}</code>
                <Copyable text={"[orchestrator]\nenabled = true"}>Copy</Copyable>
              </div>
            </Callout>
          )}
          <dl className="summary settings">
            <Row label="Agent">
              <span className="mono">
                {info.harness}
                {info.model ? ` · ${info.model}` : ""}
              </span>
            </Row>
            <Row label="Crew it starts">
              {info.dispatch === "auto" ? "Start at once" : "Wait for you to launch them"}; up to {info.maxDispatches} in a row, then it asks you
            </Row>
            <Row label="Settings">
              <Link to="/settings/missions#orchestrator">Yours, in Settings</Link>; the team's are <code>[orchestrator]</code> in <code>ws.toml</code>
            </Row>
          </dl>
          <section>
            <h2 className="group">
              On it now <span className="count">{deciding.length}</span>
            </h2>
            {deciding.length === 0 ? (
              <div className="quiet-line">Nothing right now.</div>
            ) : (
              <ul className="rows">
                {deciding.map((m) => (
                  <li key={m.slug} className="row">
                    <Link to={`/missions/${m.slug}`} className="row-link">
                      <Status s={headlineStatus(m.headline)} iconOnly />
                      <span className="title">{m.title}</span>
                      <span className="dim">
                        <Inline text={m.headline.text} lines={1} />
                      </span>
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </section>
          <section>
            <h2 className="group">
              Recent runs <span className="count">{info.runs.length}</span>
            </h2>
            {info.runs.length === 0 ? (
              <Empty icon="sparkle" title="No runs yet">
                Each time it looks at a mission shows here: what it was told, what it said, and every command it ran.
              </Empty>
            ) : (
              <RunList runs={info.runs} withMission />
            )}
          </section>
        </>
      )}
    </div>
  );
}

// RunList is runs in brief, each a link to its page.
export function RunList({ runs, withMission }: { runs: OrchestratorRun[]; withMission?: boolean }) {
  return (
    <ul className="rows runs">
      {runs.map((r) => (
        <li key={`${r.mission}/${r.id}`} className="row">
          <Link to={`/missions/${r.mission}/orchestrator/${r.id}`} className="row-link">
            <Status s={outcome(r)} iconOnly size={16} className="row-status" />
            <span className="row-main">
              <span className="row-line">
                {withMission && <span className="title">{r.missionTitle}</span>}
                <span className="dim">on {lookedAt(r)}</span>
                <span className={`chip tone-${outcome(r).tone}`}>{outcome(r).words}</span>
              </span>
              {r.said && (
                <span className="row-sub">
                  <Inline text={r.said} lines={1} />
                </span>
              )}
            </span>
            <Time iso={r.started} className="age run-time" />
          </Link>
        </li>
      ))}
    </ul>
  );
}

// outcome says how a run went: it acted, did nothing, or couldn't run.
function outcome(r: OrchestratorRun): StatusInfo {
  const refused = r.refused ? `, ${r.refused} refused` : "";
  if (r.error) return { icon: "x-circle", tone: "danger", words: `Failed${refused}` };
  if (!r.acted) return { icon: "circle-dashed", tone: "quiet", words: `Did nothing${r.retry ? " again" : ""}${refused}` };
  return { icon: "check-circle", tone: "done", words: `Acted${r.retry ? " on a second try" : ""}${refused}` };
}

export function OrchestratorRunPage({ workspace }: { workspace: Workspace }) {
  const { slug = "", id = "" } = useParams();
  const m = workspace.missions.find((m) => m.slug === slug);
  useTitle(`Orchestrator run${m ? ` · ${m.title}` : ""}`);
  const { value: d, error, reload: load } = useLoaded<OrchestratorRunDetail>(() => getOrchestratorRun(slug, id), [slug, id]);
  return (
    <div className="page narrow run-page">
      <nav className="crumbs" aria-label="Breadcrumb">
        <Link to={`/missions/${slug}`}>
          <Icon name="arrow-left" size={13} /> {m?.title ?? slug}
        </Link>
        <span className="sep">·</span>
        <Link to="/orchestrator">All runs</Link>
      </nav>
      <header className="page-head">
        <h1>Orchestrator run</h1>
        {d && <Status s={outcome(d)} className="mission-state" />}
      </header>
      {error && <ErrorCard error={error} retry={load} />}
      {!d && !error && <Skeleton rows={6} />}
      {d && (
        <>
          <div className="run-meta">
            <span>
              Looked at <strong>{lookedAt(d)}</strong>
            </span>
            <span>
              <Time iso={d.started} /> · took {duration(d.started, d.ended)}
            </span>
            <span className="mono">
              {d.harness}
              {d.model ? ` · ${d.model}` : ""}
            </span>
          </div>
          {d.retry && (
            <Callout tone="quiet" icon="rotate" title="A second try">
              The run before it ran no <code>ws mission</code> commands, so it was run again and told its shell works.
            </Callout>
          )}
          {d.error && (
            <Callout tone="danger" title="It failed">
              <Inline text={d.error} />
            </Callout>
          )}
          <section>
            <h2 className="group">What it said</h2>
            {d.output ? (
              <div className="prose">
                <Markdown text={d.output} />
              </div>
            ) : (
              <div className="quiet-line">Nothing.</div>
            )}
          </section>
          <section>
            <h2 className="group">
              What it ran <span className="count">{d.log.length}</span>
            </h2>
            {d.log.length === 0 ? <div className="quiet-line">No calls recorded: only Claude Code reports them.</div> : <Calls calls={d.log} root={workspace.root} open />}
          </section>
          <section>
            <details className="fold">
              <summary>The prompt it was given</summary>
              <div className="prose prompt">
                <Markdown text={d.prompt} />
              </div>
            </details>
          </section>
        </>
      )}
    </div>
  );
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="summary-row">
      <dt>{label}</dt>
      <dd>{children}</dd>
    </div>
  );
}
