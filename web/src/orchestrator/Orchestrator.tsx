import { useMemo } from "react";
import { Crumbs } from "../ui/Crumbs";
import { Link, useParams } from "react-router";
import { getOrchestrator, getOrchestratorRun, type OrchestratorInfo, type OrchestratorRun, type OrchestratorRunDetail, type Workspace } from "../api";
import { Markdown } from "../markdown/LazyMarkdown";
import { Calls } from "../ui/Calls";
import { Inline } from "../ui/inline";
import { useTitle } from "../ui/useTitle";
import { Skeleton } from "../ui/Skeleton";
import { Empty } from "../ui/Empty";
import { ErrorCard } from "../ui/ErrorCard";
import { Callout } from "../ui/Callout";
import { Copyable } from "../ui/Copyable";
import { Status, type StatusInfo } from "../ui/Status";
import { headlineStatus } from "../ui/statusOf";
import { duration, Time } from "../ui/time";
import { useLoaded } from "../load";
import { Chip } from "../ui/Chip";
import { PageHead } from "../ui/PageHead";
import { GroupHead } from "../ui/GroupHead";
import { Summary, SummaryRow } from "../ui/Summary";
import { Row, RowLine, RowMain, RowSub, RowTitle, Rows, rowLinkClass, rowStatusClass } from "../ui/Rows";
import { useWorkspace } from "../workspace/store";
import { useThingMenu } from "../shell/ThingMenu";

// The orchestrator's page: whether it's on and how it's set up, the
// missions it's on now, and its latest runs; each run's own page has what
// it was told, what it said and every call it made, refused ones marked.

// changedKey is when anything about the missions last changed: pages here
// load again when it moves (a run's end is logged on its mission).
function changedKey(missions: Workspace["missions"]) {
  return missions.map((m) => m.updated).join("|");
}

// lookedAt is what a run looked at, its trigger said once: "c1 (done)".
export const lookedAt = (r: Pick<OrchestratorRun, "for" | "trigger">) =>
  !r.trigger || r.for.includes(`(${r.trigger})`) ? r.for : `${r.for} (${r.trigger})`;

export function OrchestratorPage() {
  const menu = useThingMenu();
  const missions = useWorkspace((w) => w.missions);
  useTitle("Orchestrator");
  const key = useMemo(() => changedKey(missions), [missions]);
  const { value: info, error, reload: load } = useLoaded<OrchestratorInfo>(getOrchestrator, [key], { keep: true });
  const deciding = missions.filter((m) => m.headline.kind === "deciding");
  const on: StatusInfo | undefined = info && (info.problem ? { icon: "x-circle", tone: "danger", words: "Can't run" } : info.enabled ? { icon: "check-circle", tone: "done", words: "On" } : { icon: "pause", tone: "quiet", words: "Off" });
  return (
    <div className="page">
      <PageHead title="Orchestrator">
        {on && <Status s={on} />}
      </PageHead>
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
          <Summary className="orchestrator-facts">
            <SummaryRow label="Agent">
              <span className="mono">
                {info.harness}
                {info.model ? ` · ${info.model}` : ""}
              </span>
            </SummaryRow>
            <SummaryRow label="Crew it starts">
              {info.dispatch === "auto" ? "Start at once" : "Wait for you to launch them"}; up to {info.maxDispatches} in a row, then it asks you
            </SummaryRow>
            <SummaryRow label="Settings">
              <Link to="/settings/missions#orchestrator">Yours, in Settings</Link>; the team's are <code>[orchestrator]</code> in <code>ws.toml</code>
            </SummaryRow>
          </Summary>
          <section>
            <GroupHead count={deciding.length}>
              On it now
            </GroupHead>
            {deciding.length === 0 ? (
              <div className="quiet-line">Nothing right now.</div>
            ) : (
              <Rows>
                {deciding.map((m) => (
                  <Row key={m.slug}>
                    <Link to={`/missions/${m.slug}`} className={rowLinkClass()} onContextMenu={menu(`/missions/${m.slug}`, m.title)}>
                      <Status s={headlineStatus(m.headline)} iconOnly />
                      <RowTitle>{m.title}</RowTitle>
                      <span className="dim">
                        <Inline text={m.headline.text} lines={1} />
                      </span>
                    </Link>
                  </Row>
                ))}
              </Rows>
            )}
          </section>
          <section>
            <GroupHead count={info.runs.length}>
              Recent runs
            </GroupHead>
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
  const menu = useThingMenu();
  return (
    <Rows className="runs">
      {runs.map((r) => (
        <Row key={`${r.mission}/${r.id}`}>
          <Link to={`/missions/${r.mission}/orchestrator/${r.id}`} className={rowLinkClass("orch-run-link")} onContextMenu={menu(`/missions/${r.mission}/orchestrator/${r.id}`)}>
            <Status s={outcome(r)} iconOnly size={16} className={rowStatusClass()} />
            <RowMain>
              <RowLine>
                {withMission && <RowTitle>{r.missionTitle}</RowTitle>}
                <span className="dim">on {lookedAt(r)}</span>
                <Chip tone={outcome(r).tone}>{outcome(r).words}</Chip>
              </RowLine>
              {r.said && (
                <RowSub>
                  <Inline text={r.said} lines={1} />
                </RowSub>
              )}
            </RowMain>
            <Time iso={r.started} className="age run-time" />
          </Link>
        </Row>
      ))}
    </Rows>
  );
}

// outcome says how a run went: it acted, did nothing, or couldn't run.
function outcome(r: OrchestratorRun): StatusInfo {
  const refused = r.refused ? `, ${r.refused} refused` : "";
  if (r.error) return { icon: "x-circle", tone: "danger", words: `Failed${refused}` };
  if (!r.acted) return { icon: "circle-dashed", tone: "quiet", words: `Did nothing${r.retry ? " again" : ""}${refused}` };
  return { icon: "check-circle", tone: "done", words: `Acted${r.retry ? " on a second try" : ""}${refused}` };
}

export function OrchestratorRunPage() {
  const { slug = "", id = "" } = useParams();
  const m = useWorkspace((w) => w.missions.find((m) => m.slug === slug));
  const root = useWorkspace((w) => w.root);
  useTitle(`Orchestrator run${m ? ` · ${m.title}` : ""}`);
  const { value: d, error, reload: load } = useLoaded<OrchestratorRunDetail>(() => getOrchestratorRun(slug, id), [slug, id]);
  return (
    <div className="page narrow run-page">
      <Crumbs
        steps={[
          { to: `/missions/${slug}`, label: m?.title ?? slug },
          { to: "/orchestrator", label: "All runs" },
        ]}
      />
      <PageHead title="Orchestrator run">
        {d && <Status s={outcome(d)} />}
      </PageHead>
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
            <GroupHead>What it said</GroupHead>
            {d.output ? (
              <div className="prose">
                <Markdown text={d.output} />
              </div>
            ) : (
              <div className="quiet-line">Nothing.</div>
            )}
          </section>
          <section>
            <GroupHead count={d.log.length}>
              What it ran
            </GroupHead>
            {d.log.length === 0 ? <div className="quiet-line">No calls recorded: only Claude Code reports them.</div> : <Calls calls={d.log} root={root} open />}
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

