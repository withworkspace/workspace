import type { Stage } from "../api";
import { MissionPlanNote } from "./MissionPlanNote";

// StepLimit is a Work step's clock: its limit, when it runs out, or that
// it has.
export function StepLimit({ s }: { s: Stage }) {
  if (s.outOfTime) {
    return (
      <MissionPlanNote icon="clock" late title={`Its limit (${s.limit}) ran out while its crew were at it`}>
        out of time
      </MissionPlanNote>
    );
  }
  const at = s.deadline ? new Date(s.deadline) : null;
  return (
    <MissionPlanNote icon="clock" title={`Its limit: ${s.limit}. ws holds the clock from when crew start on it.`}>
      {at ? `until ${at.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}` : (s.limit ?? "").split(/[,;]/)[0]}
    </MissionPlanNote>
  );
}
