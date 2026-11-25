import type { Stage } from "../api";
import { MissionPlan } from "./MissionPlan";
import { MissionPlanCheckList } from "./MissionPlanCheckList";
import { MissionPlanStep } from "./MissionPlanStep";
import { checksOf } from "./planChecks";
import { stepNotes } from "./stepNotes";

// Stepper is a mission's stages as its Summary has them: a MissionPlan,
// with what's to be known of each step worked out here.
export function Stepper({ stages }: { stages: Stage[] }) {
  return (
    <MissionPlan>
      {stages.map((s) => (
        <MissionPlanStep
          key={s.n}
          n={s.n}
          title={s.title}
          state={s.state}
          hint={s.type ? `${s.type}: ${s.text}` : s.text}
          why={s.why}
          under={s.round && s.state !== "accepted" ? <MissionPlanCheckList checks={checksOf(s)} /> : null}
        >
          {stepNotes(s)}
        </MissionPlanStep>
      ))}
    </MissionPlan>
  );
}
