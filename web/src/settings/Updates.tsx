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
        if (now.error) toast(`Couldn't look: ${humanError(now.error)}`, { tone: "danger", ms: 8000 });
        else toast(now.newer ? `ws ${now.latest} is out` : "No later release of ws", { tone: "done" });
      })
      .catch((e: Error) => toast(`Couldn't look: ${humanError(e.message)}`, { tone: "danger", ms: 8000 }))
      .finally(() => setBusy(undefined));
  };
  const upgrade = async () => {
    if (!up?.latest) return;
    const yes = await confirm({
      title: `Upgrade ws to ${up.latest}?`,
      body: `It's fetched, checked against the release's checksums, and put in place of ${shortVersion(up.version)}, which is kept: ws upgrade --rollback puts it back. The daemon restarts onto it once nothing's at work (no crew mid-turn, no agent's run), and this page carries on.`,
      confirm: "Upgrade",
    });
    if (!yes) return;
    setBusy("upgrade");
    try {
      const did = await upgradeWS();
      toast(did.installed ? `ws ${did.to} is installed: the daemon restarts onto it once nothing's at work` : `ws ${did.to} is what's running already`, { tone: "done", ms: 8000 });
      await reload();
    } catch (e) {
      toast(`Couldn't upgrade: ${humanError((e as Error).message)}`, { tone: "danger", ms: 10000 });
    } finally {
      setBusy(undefined);
    }
  };

  if (error && !up) return <ErrorCard error={error} retry={reload} />;
  if (!up) return <Skeleton rows={2} />;
  return <SettingsUpdates up={up} standing={standingOf(up)} busy={busy} onLook={look} onUpgrade={() => void upgrade()} />;
}
