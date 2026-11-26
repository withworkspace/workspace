import type { Stage } from "../api";
import type { IconName } from "../ui/Icon";
import { MissionPlanNote } from "./MissionPlanNote";

// StepWho is who's on a step of the plan that isn't crew's: yours (a
// checkpoint, which only you pass: a review, a decision, a landing), ws's
// (it opens the pull request), CI's, someone outside's, the planner's.
export function StepWho({ s }: { s: Stage }) {
  const yours: Record<string, [string, string]> = {
    Decide: ["yours, to decide", "Yours to decide: your answer is recorded and passes the step"],
    Land: ["yours, to land", "Yours to land: passes once landed"],
  };
  const [icon, label, title]: [IconName, string, string] =
    s.who === "you"
      ? ["user", ...(yours[s.type ?? ""] ?? ["yours", "Checkpoint: review the earlier steps' work. Only you can pass."])]
      : s.who === "ws"
        ? ["pr", "Workspace opens", "Workspace opens the pull request; passes when merged"]
        : s.who === "the planner"
          ? ["sparkle", "planner", "Planner: writes the steps that follow"]
          : ["clock", s.who === "CI" ? "CI" : "outside", `Waited for: ${s.whoSays ?? s.who}`];
  return (
    <MissionPlanNote icon={icon} yours={s.who === "you"} title={title}>
      {label}
    </MissionPlanNote>
  );
}
