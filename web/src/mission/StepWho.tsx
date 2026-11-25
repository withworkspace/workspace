import type { Stage } from "../api";
import type { IconName } from "../ui/Icon";
import { MissionPlanNote } from "./MissionPlanNote";

// StepWho is who's on a step of the plan that isn't crew's: yours (a
// checkpoint, which only you pass: a review, a decision, a landing), ws's
// (it opens the pull request), CI's, someone outside's, the planner's.
export function StepWho({ s }: { s: Stage }) {
  const yours: Record<string, [string, string]> = {
    Decide: ["yours, to decide", "Yours to decide: your answer is recorded as a decision, and passes the step"],
    Land: ["yours, to land", "Yours to land: the step passes once it's landed"],
  };
  const [icon, label, title]: [IconName, string, string] =
    s.who === "you"
      ? ["user", ...(yours[s.type ?? ""] ?? ["yours", "A checkpoint: you look at what the steps before it made, and only you pass it"])]
      : s.who === "ws"
        ? ["pr", "ws opens it", "ws opens the pull request itself, and passes the step when it merges"]
        : s.who === "the planner"
          ? ["sparkle", "planner", "The planner's: it writes the steps after it"]
          : ["clock", s.who === "CI" ? "CI" : "outside", `Waited for: ${s.whoSays ?? s.who}`];
  return (
    <MissionPlanNote icon={icon} yours={s.who === "you"} title={title}>
      {label}
    </MissionPlanNote>
  );
}
