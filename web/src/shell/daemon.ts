import { shutDownDaemon } from "../api";
import { toast } from "../ui/toast";
import { humanError } from "../ui/strings";
import { confirm } from "../ui/Confirm";
import { useChromeStore } from "../chrome/store";

// askShutDown shuts the workspace's daemon down, once you've said yes:
// every crew it holds is hung up, and this page goes with it (the window
// is told, chrome/store.ts, and says it's gone). It's in the workspace's
// menu, in Find, and on Settings, under About.
export async function askShutDown() {
  const yes = await confirm({
    title: "Shut down Workspace?",
    body: "Stops all crew and closes this page. Run ws web to start again.",
    confirm: "Shut down",
    danger: true,
  });
  if (!yes) return;
  try {
    await shutDownDaemon();
    useChromeStore.getState().daemonClosed();
  } catch (e) {
    toast(`Could not shut down Workspace: ${humanError(String(e))}`, { tone: "danger" });
  }
}
