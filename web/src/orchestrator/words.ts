import type { OrchestratorRun, Workspace } from "../api";
import type { StatusInfo } from "../ui/Status";

// changedKey is when anything about the missions last changed: pages here
// load again when it moves (a run's end is logged on its mission).
export function changedKey(missions: Workspace["missions"]) {
  return missions.map((m) => m.updated).join("|");
}

// lookedAt is what a run looked at, its trigger said once: "c1 (done)".
export const lookedAt = (r: Pick<OrchestratorRun, "for" | "trigger">) =>
  !r.trigger || r.for.includes(`(${r.trigger})`) ? r.for : `${r.for} (${r.trigger})`;

// outcome says how a run went: it acted, did nothing, or couldn't run.
export function outcome(r: { acted: boolean; error?: string; retry?: boolean; refused?: number }): StatusInfo {
  const refused = r.refused ? `, ${r.refused} refused` : "";
  if (r.error) return { icon: "x-circle", tone: "danger", words: `Failed${refused}` };
  if (!r.acted) return { icon: "circle-dashed", tone: "quiet", words: `Did nothing${r.retry ? " again" : ""}${refused}` };
  return { icon: "check-circle", tone: "done", words: `Acted${r.retry ? " on a second try" : ""}${refused}` };
}

// agentOf is the agent a run, or the orchestrator, is on: its harness, and
// its model if it says one.
export const agentOf = (a: { harness: string; model?: string }) => `${a.harness}${a.model ? ` · ${a.model}` : ""}`;
