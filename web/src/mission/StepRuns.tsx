import type { Stage } from "../api";
import { MissionPlanNote } from "./MissionPlanNote";

// StepRuns is where a step that runs in several repos runs: the repos
// once they're named, each done or not, or what its words say before.
export function StepRuns({ s }: { s: Stage }) {
  const runs = s.runs ?? [];
  if (!runs.length) {
    return (
      <MissionPlanNote icon="layers" title={`In each ${s.inEach}: the orchestrator names the repos when it starts`}>
        in each {s.inEach}
      </MissionPlanNote>
    );
  }
  const done = runs.filter((r) => r.done).length;
  return (
    <MissionPlanNote icon="layers" title={runs.map((r) => `${r.repo}${r.done ? ": done" : ""}`).join("\n")}>
      {runs.length === done ? `in ${runs.length} repos` : `${done} of ${runs.length} repos done`}
    </MissionPlanNote>
  );
}
