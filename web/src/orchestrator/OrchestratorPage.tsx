import { useMemo } from "react";
import { getOrchestrator, type OrchestratorInfo } from "../api";
import { useLoaded } from "../load";
import { useThingMenu } from "../shell/ThingMenu";
import { Callout } from "../ui/Callout";
import { Empty } from "../ui/Empty";
import { ErrorCard } from "../ui/ErrorCard";
import { GroupHead } from "../ui/GroupHead";
import { Inline } from "../ui/Inline";
import { PageHead } from "../ui/PageHead";
import { Rows } from "../ui/Rows";
import { Skeleton } from "../ui/Skeleton";
import { Status, type StatusInfo } from "../ui/Status";
import { headlineStatus } from "../ui/statusOf";
import { useTitle } from "../ui/useTitle";
import { useWorkspace } from "../workspace/store";
import { OrchestratorFacts } from "./OrchestratorFacts";
import { OrchestratorLede } from "./OrchestratorLede";
import { OrchestratorMissionRow } from "./OrchestratorMissionRow";
import { OrchestratorNone } from "./OrchestratorNone";
import { OrchestratorOff } from "./OrchestratorOff";
import { OrchestratorRunRow } from "./OrchestratorRunRow";
import { agentOf, changedKey, lookedAt, outcome } from "./words";

// The orchestrator's page: whether it's on and how it's set up, the
// missions it's on now, and its latest runs; each run's own page has what
// it was told, what it said and every call it made, refused ones marked
// (OrchestratorRunPage). This is what the page knows; its parts are the
// Orchestrator… components beside it.
export function OrchestratorPage() {
  const menu = useThingMenu();
  const missions = useWorkspace((w) => w.missions);
  useTitle("Orchestrator");
  const key = useMemo(() => changedKey(missions), [missions]);
  const { value: info, error, reload: load } = useLoaded<OrchestratorInfo>(getOrchestrator, [key], { keep: true });
  const deciding = missions.filter((m) => m.headline.kind === "deciding");
  const on: StatusInfo | undefined = info && (info.problem ? { icon: "x-circle", tone: "danger", words: "Cannot run" } : info.enabled ? { icon: "check-circle", tone: "done", words: "On" } : { icon: "pause", tone: "quiet", words: "Off" });
  return (
    <div className="page">
      <PageHead title="Orchestrator">{on && <Status s={on} />}</PageHead>
      <OrchestratorLede>
        An agent that moves missions on between crew. When crew finish or go quiet, you answer or review, or a
        PR changes, it reads the mission and accepts a stage, dispatches the next crew, or asks you.
      </OrchestratorLede>
      {error && <ErrorCard error={error} retry={load} />}
      {!info && !error && <Skeleton rows={5} />}
      {info && (
        <>
          {info.problem && (
            <Callout tone="danger" title="Cannot run with current setup">
              <Inline text={info.problem} />
            </Callout>
          )}
          {!info.enabled && <OrchestratorOff />}
          <OrchestratorFacts agent={agentOf(info)} auto={info.dispatch === "auto"} most={info.maxDispatches} />
          <section>
            <GroupHead count={deciding.length}>Current missions</GroupHead>
            {deciding.length === 0 ? (
              <OrchestratorNone>Nothing right now.</OrchestratorNone>
            ) : (
              <Rows>
                {deciding.map((m) => (
                  <OrchestratorMissionRow key={m.slug} to={`/missions/${m.slug}`} title={m.title} status={headlineStatus(m.headline)} headline={m.headline.text} onContextMenu={menu(`/missions/${m.slug}`, m.title)} />
                ))}
              </Rows>
            )}
          </section>
          <section>
            <GroupHead count={info.runs.length}>Recent runs</GroupHead>
            {info.runs.length === 0 ? (
              <Empty icon="sparkle" title="No runs yet">
                Each run shows here, with its prompt, reply and commands.
              </Empty>
            ) : (
              <Rows>
                {info.runs.map((r) => {
                  const to = `/missions/${r.mission}/orchestrator/${r.id}`;
                  return <OrchestratorRunRow key={`${r.mission}/${r.id}`} to={to} outcome={outcome(r)} mission={r.missionTitle} on={lookedAt(r)} said={r.said} started={r.started} onContextMenu={menu(to)} />;
                })}
              </Rows>
            )}
          </section>
        </>
      )}
    </div>
  );
}
