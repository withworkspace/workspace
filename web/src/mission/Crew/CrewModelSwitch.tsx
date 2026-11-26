import { useState } from "react";
import { type Crew, switchCrewModel } from "../../api";
import { useLoaded } from "../../load";
import { modelsOf } from "../../models/models";
import { ModelSwitch } from "../../models/ModelSwitch";
import { humanError } from "../../ui/strings";
import { toast } from "../../ui/toast";

// CrewModelSwitch is running crew's model, switched without ending them:
// over ACP their agent's session takes the model from its next turn; in a
// terminal, its harness's command (/model) is typed in, which is only
// done while they're waiting, not in the middle of a turn.
export function CrewModelSwitch({ slug, crew }: { slug: string; crew: Crew }) {
  const got = useLoaded(() => modelsOf(crew.harness), [crew.harness]);
  const [busy, setBusy] = useState(false);
  const working = !crew.structured && crew.activity === "working";
  return (
    <ModelSwitch
      models={got.value ?? []}
      asking={got.value === undefined && !got.error}
      value={crew.model ?? ""}
      placeholder={`${crew.harness}'s default`}
      busy={busy}
      unavailable={working ? `${crew.id} is working: switch model when the turn ends` : undefined}
      aria-label={`${crew.id}'s model`}
      onSwitch={(model) => {
        setBusy(true);
        switchCrewModel(slug, crew.id, model).then(
          (r) => toast(r.message, { tone: "done" }),
          (e: Error) => toast(humanError(e.message), { tone: "danger" }),
        ).finally(() => setBusy(false));
      }}
    />
  );
}
