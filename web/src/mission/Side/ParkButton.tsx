import { useState } from "react";
import { act, type MissionDetail } from "../../api";
import type { Reload } from "../../load";
import { Button } from "../../ui/Button";
import { Icon } from "../../ui/Icon";
import { humanError } from "../../ui/strings";
import { toast } from "../../ui/toast";

// ParkButton puts a mission aside, out of what needs you and out of the
// orchestrator's way, until it's unparked; and unparks a parked one.
export function ParkButton({ d, reload }: { d: MissionDetail; reload: Reload }) {
  const [busy, setBusy] = useState(false);
  if (d.state === "archived" || d.state === "briefing") return null;
  const parked = d.state === "parked";
  const run = () => {
    setBusy(true);
    act(d.slug, parked ? "unpark" : "park")
      .then(
        (r) => {
          toast(r.message, { tone: "done" });
          return reload();
        },
        (e: Error) => toast(humanError(e.message), { tone: "danger" }),
      )
      .finally(() => setBusy(false));
  };
  return (
    <Button variant="ghost" size="small" disabled={busy} onClick={run} title={parked ? "Take it off parked" : "Put it aside until you unpark it"}>
      <Icon name={parked ? "play" : "pause"} size={12} /> {parked ? "Unpark" : "Park"}
    </Button>
  );
}
