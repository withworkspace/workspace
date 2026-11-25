import { useState } from "react";
import { getTalkModel, setTalkModel } from "../api";
import { useLoaded } from "../load";
import { modelsOf } from "../models/models";
import { ModelSwitch } from "../models/ModelSwitch";
import { humanError } from "../ui/strings";
import { toast } from "../ui/toast";

// TalkModelSwitch is the model a conversation's turns run on, switched for
// it alone: its next turn starts on it, its session going on. Until one's
// chosen it's its role's (Settings' conversation model), which choosing
// that again goes back to. chosen is what the conversation records,
// to read again when it changes.
export function TalkModelSwitch({ slug, id, chosen }: { slug: string; id: string; chosen?: string }) {
  const on = useLoaded(() => getTalkModel(slug, id), [slug, id, chosen]);
  const harness = on.value?.harness;
  const got = useLoaded(harness ? () => modelsOf(harness) : null, [harness]);
  const [busy, setBusy] = useState(false);
  if (!on.value) return null;
  return (
    <ModelSwitch
      models={got.value ?? []}
      asking={got.value === undefined && !got.error}
      value={on.value.model}
      placeholder={`${on.value.harness}'s default`}
      busy={busy}
      aria-label="This conversation's model"
      onSwitch={(model) => {
        setBusy(true);
        setTalkModel(slug, id, model).then(
          (m) => {
            on.set(m);
            toast(`Its next turn runs on ${m.model}.`, { tone: "done" });
          },
          (e: Error) => toast(humanError(e.message), { tone: "danger" }),
        ).finally(() => setBusy(false));
      }}
    />
  );
}
