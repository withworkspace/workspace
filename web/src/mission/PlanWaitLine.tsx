import { Chip } from "../ui/Chip";
import { MissionPlanLine } from "./MissionPlanLine";
import type { WaitHow } from "./planRows";

// PlanWaitLine says how a Wait is passed: watched by ws, or not at all.
export function PlanWaitLine({ how }: { how: WaitHow }) {
  return (
    <MissionPlanLine>
      <Chip tone={how.auto ? "progress" : "attention"}>{how.word}</Chip>
      <span>{how.says}</span>
    </MissionPlanLine>
  );
}
