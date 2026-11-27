import { act, dockPR, mergePR, restoreArtifact, setTalkStatus } from "../api";
import { archiveMission } from "../mission/archive";
import { confirm } from "../ui/Confirm";
import { toast } from "../ui/toast";
import { humanError } from "../ui/strings";
import type { WsDoing } from "./acts";
import type { Doing } from "./find";
import { pinNamed, unpinNamed } from "./pins";

// What ws does to one of its own things, when it's chosen: from a row of
// Find, or from the menu of a row anywhere (ThingMenu). One place does
// it, so both say the same and ask the same before what can't be taken
// back.

// ThingDoing is something done to a thing of ws's own (acts.ts): a
// mission's act, or one of ws's to a conversation, an artifact or a pull
// request, or any of them pinned or unpinned.
export type ThingDoing = Extract<Doing, { act: unknown } | { ws: unknown }>;

// isThingDoing is whether it's one of those.
export const isThingDoing = (d: Doing): d is ThingDoing => "act" in d || "ws" in d;

// stopCrew stops a crew, once you've said so, as its own page asks.
async function stopCrew(slug: string, crew: string) {
  const yes = await confirm({
    title: `Stop ${crew}?`,
    body: "Work in progress is lost. Session can be resumed later, if recorded.",
    confirm: `Stop ${crew}`,
    danger: true,
  });
  return yes ? act(slug, "stop", { crew }) : undefined;
}

// doWs does what ws does to one of its things that isn't a mission's
// act or a pin, and says what it did.
const doWs = (d: Exclude<WsDoing, { pin: string } | { unpin: string }>): Promise<string> =>
  "talk" in d
    ? setTalkStatus("", d.talk, d.status).then((t) => (t.status === "resolved" ? "Resolved." : "Reopened."))
    : "restore" in d
      ? restoreArtifact(d.restore).then(() => "Restored.")
      : "merge" in d
        ? mergePR(d.merge, { when: d.when, head: d.head }).then((r) => r.message)
        : dockPR(d.dock).then((r) => r.message);

// doToThing does it, and says in a toast what was done, or what went
// wrong. A pin says nothing: it's there in the sidebar. An unpin says
// so, with the way back (pins.ts).
export function doToThing(d: ThingDoing) {
  const wrong = (e: Error) => toast(humanError(e.message), { tone: "danger" });
  if ("ws" in d) {
    const ws = d.ws;
    if ("pin" in ws) pinNamed(ws.pin).catch(wrong);
    else if ("unpin" in ws) unpinNamed(ws.unpin).catch(wrong);
    else doWs(ws).then((said) => toast(said, { tone: "done" }), wrong);
  } else
    (d.act === "archive"
      ? archiveMission(d.slug)
      : d.act === "stop"
        ? stopCrew(d.slug, d.crew ?? "")
        : // A merge's target is the pull request, which the headline names.
          act(d.slug, d.act, d.act.startsWith("merge") ? { pr: d.crew } : { crew: d.crew })
    ).then((r) => r && toast(r.message, { tone: "done" }), wrong);
}
