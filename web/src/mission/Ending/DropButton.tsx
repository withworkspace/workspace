import { useState } from "react";
import { act, type MissionDetail } from "../../api";
import type { Reload } from "../../load";
import { humanError } from "../../ui/strings";
import { toast } from "../../ui/toast";
import { MissionDrop } from "../MissionDrop";
import { ended } from "./ended";

// DropButton gives the mission up, from beside its state: it ends
// dropped, with why if you say. The one ending that's yours to begin:
// every other is suggested, or ws's own.
export function DropButton({ d, reload }: { d: MissionDetail; reload: Reload }) {
  const [busy, setBusy] = useState(false);
  if (d.ending || d.state === "archived") return null;
  const drop = (why: string) => {
    setBusy(true);
    act(d.slug, "end", { ending: "dropped", text: why })
      .then(
        (r) => ended(d.slug, r.message, reload),
        (e: Error) => toast(humanError(e.message), { tone: "danger" }),
      )
      .finally(() => setBusy(false));
  };
  return <MissionDrop busy={busy} onDrop={drop} />;
}
