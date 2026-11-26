import { useState } from "react";
import { checkUpdates, getUpdates, type UpdatesInfo, upgradeWS, useEventsOf } from "../api";
import { useLoaded } from "../load";
import { shortVersion } from "../shell/version";
import { confirm } from "../ui/Confirm";
import { ErrorCard } from "../ui/ErrorCard";
import { Skeleton } from "../ui/Skeleton";
import { humanError } from "../ui/strings";
import { toast } from "../ui/toast";
import { SettingsUpdates } from "./SettingsUpdates";
import { standingOf } from "./standing";

// Updates, on Settings' About: the latest release of ws as the daemon
// last found it (it looks once a day, and when Check now is pressed),
// against the one that's running. A later one is installed from here, as
// ws upgrade does it, where this ws is a release's build; the daemon
// restarts onto it by itself once nothing's at work. This is what's read
// and done; SettingsUpdates draws it.
export function Updates({ running }: { running?: string }) {
  // The daemon restarting onto another ws is this page's to hear of: what
  // it says of updates is asked again then.
  const { value: up, error, set, reload } = useLoaded<UpdatesInfo>(getUpdates, [running], { keep: true });
  const [busy, setBusy] = useState<"look" | "upgrade">();
  const waiting = !!up?.installed;
  // While a ws waits to be restarted onto, what it's waiting for (crew
  // mid-turn, an agent's run) changes with the workspace: read again when
  // it does.
  useEventsOf(waiting ? ["workspace"] : [], { changed: () => void reload() });

  const look = () => {
    setBusy("look");
    checkUpdates()
      .then((now) => {
        set(now);
        if (now.error) toast(`Could not check for updates: ${humanError(now.error)}`, { tone: "danger", ms: 8000 });
        else toast(now.newer ? `ws ${now.latest} is out` : "ws is up to date", { tone: "done" });
      })
      .catch((e: Error) => toast(`Could not check for updates: ${humanError(e.message)}`, { tone: "danger", ms: 8000 }))
      .finally(() => setBusy(undefined));
  };
  const upgrade = async () => {
    if (!up?.latest) return;
    const yes = await confirm({
      title: `Upgrade ws to ${up.latest}?`,
      body: `Replaces ${shortVersion(up.version)}, which is kept: ws upgrade --rollback restores it. Workspace restarts when idle (no crew mid-turn, no agent runs); this page carries on.`,
      confirm: "Upgrade",
    });
    if (!yes) return;
    setBusy("upgrade");
    try {
      const did = await upgradeWS();
      toast(did.installed ? `ws ${did.to} installed: Workspace restarts when idle` : `Already running ws ${did.to}`, { tone: "done", ms: 8000 });
      await reload();
    } catch (e) {
      toast(`Could not upgrade: ${humanError((e as Error).message)}`, { tone: "danger", ms: 10000 });
    } finally {
      setBusy(undefined);
    }
  };

  if (error && !up) return <ErrorCard error={error} retry={reload} />;
  if (!up) return <Skeleton rows={2} />;
  return <SettingsUpdates up={up} standing={standingOf(up)} busy={busy} onLook={look} onUpgrade={() => void upgrade()} />;
}
