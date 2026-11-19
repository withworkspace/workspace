import { shutDownDaemon } from "../api";
import { confirm, humanError, toast } from "../ui/kit";

// askShutDown shuts the workspace's daemon down, once you've said yes:
// every crew it holds is hung up, and this page goes with it (closed says
// so, for the page to say it's gone). It's in the workspace's menu, and
// on Settings, under About.
export async function askShutDown(closed: () => void) {
  const yes = await confirm({
    title: "Shut down the daemon?",
    body: "Every crew it holds is hung up, as ws daemon stop does, and this page closes with it. ws web starts it again.",
    confirm: "Shut down",
    danger: true,
  });
  if (!yes) return;
  try {
    await shutDownDaemon();
    closed();
  } catch (e) {
    toast(`Couldn't shut the daemon down: ${humanError(String(e))}`, { tone: "danger" });
  }
}
