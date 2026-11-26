import { useState } from "react";
import { addPlugin } from "../../api";
import type { Reload } from "../../load";
import { toast } from "../../ui/toast";
import { changedSaid } from "../installed";
import { type Failed, failed } from "./failed";
import { PluginsAdd } from "./PluginsAdd";

// AddPlugin adds a plugin from what's typed: the repo it's published at,
// or a folder on this machine.
export function AddPlugin({ reload, onFailed }: { reload: Reload; onFailed: (f?: Failed) => void }) {
  const [from, setFrom] = useState("");
  const [busy, setBusy] = useState(false);
  const typed = from.trim();
  const add = () => {
    if (!typed || busy) return;
    setBusy(true);
    onFailed(undefined);
    addPlugin(typed)
      .then(async (c) => {
        setFrom("");
        await reload();
        toast(changedSaid(c, "added"), { tone: "done", ms: 6000 });
      })
      .catch(async (e: Error) => {
        // One that's installed and didn't start is in the list, failed.
        await reload();
        onFailed(failed(`Could not add ${typed}`, e));
      })
      .finally(() => setBusy(false));
  };
  return <PluginsAdd value={from} onChange={setFrom} busy={busy} onAdd={add} />;
}
