import type { Stage } from "../api";
import type { StepCheck } from "./MissionPlanCheckList";

// checksOf is what a Work step's checks came to in their latest round,
// those not met first, each with why it wasn't and where all it printed
// is kept.
export function checksOf(s: Stage): StepCheck[] {
  const order = { unmet: 0, "can't tell": 1, yours: 1, met: 2 } as const;
  return [...(s.checks ?? [])]
    .sort((a, b) => order[a.result ?? "met"] - order[b.result ?? "met"])
    .map((c) => ({ text: c.text, result: c.result, why: c.evidence?.split("\n")[0], kept: c.kept }));
}
