import { act, type MissionDetail, type PutAwayItem } from "../../api";
import type { Reload } from "../../load";
import { humanError } from "../../ui/strings";
import { toast } from "../../ui/toast";
import { MissionPuttingAway } from "./MissionPuttingAway";

// SummaryPuttingAway is what archiving the mission left to do behind, on
// its summary: a capsule that couldn't be burned is tried again, or left
// as it is, from here.
export function SummaryPuttingAway({ d, reload }: { d: MissionDetail; reload: Reload }) {
  const items = d.puttingAway ?? [];
  const run = (verb: "put-away-retry" | "put-away-drop") => (it: PutAwayItem) =>
    act(d.slug, verb, { item: it.id }).then(
      (r) => {
        toast(r.message, { tone: "done" });
        return reload();
      },
      (e: Error) => toast(humanError(e.message), { tone: "danger" }),
    );
  return <MissionPuttingAway items={items} onRetry={run("put-away-retry")} onLeave={run("put-away-drop")} />;
}
