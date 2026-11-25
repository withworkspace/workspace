import type { Stage } from "../api";
import { MissionPlanNote } from "./MissionPlanNote";

// StepChecks is how many of a Work step's checks are met, or how many it
// has before they've been run.
export function StepChecks({ s }: { s: Stage }) {
  const checks = s.checks ?? [];
  const met = checks.filter((c) => c.result === "met").length;
  // A Verify step is held up by what's unmet: what its verifier left for
  // you (can't tell, yours) is yours when you look at its bundle.
  const short = s.type === "Verify" ? checks.some((c) => c.result === "unmet") : met < checks.length;
  const title = checks.map((c) => `${c.result ? `${c.result}: ` : ""}${c.text}`).join("\n");
  if (!s.round) {
    return (
      <MissionPlanNote icon="list" title={`Done when these are true:\n${title}`}>
        {checks.length} {checks.length === 1 ? "check" : "checks"}
      </MissionPlanNote>
    );
  }
  return (
    <MissionPlanNote icon={short ? "alert" : "check"} late={short} title={`Round ${s.round}:\n${title}`}>
      {met} of {checks.length} met
    </MissionPlanNote>
  );
}
