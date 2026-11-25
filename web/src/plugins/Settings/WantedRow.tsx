import { useState } from "react";
import { addPlugin, type WantedPlugin } from "../../api";
import type { Reload } from "../../load";
import { Button } from "../../ui/Button";
import { Icon } from "../../ui/Icon";
import { Spinner } from "../../ui/Spinner";
import { toast } from "../../ui/toast";
import { changedSaid, wantedFrom } from "../installed";
import { type Failed, failed } from "./failed";
import { PluginsRow } from "./PluginsRow";

// WantedRow is one of the team's plugins that isn't installed here, with
// the way to add it.
export function WantedRow({ w, reload, onFailed }: { w: WantedPlugin; reload: Reload; onFailed: (f?: Failed) => void }) {
  const [busy, setBusy] = useState(false);
  const add = () => {
    setBusy(true);
    onFailed(undefined);
    addPlugin(wantedFrom(w))
      .then(async (c) => {
        await reload();
        toast(changedSaid(c, "added"), { tone: "done", ms: 6000 });
      })
      .catch(async (e: Error) => {
        await reload();
        onFailed(failed(`Couldn't add ${w.name}`, e));
      })
      .finally(() => setBusy(false));
  };
  return (
    <PluginsRow
      kind="wanted"
      icon="circle-dashed"
      name={w.name}
      version={w.version}
      source={w.source}
      actions={
        <Button size="small" onClick={add} disabled={busy || !w.source} title={`Install it from ${w.source}`}>
          {busy ? <Spinner size={11} /> : <Icon name="plus" size={11} />} {busy ? "Adding…" : "Add"}
        </Button>
      }
    />
  );
}
