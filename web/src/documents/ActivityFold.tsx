import type { SpaceActivity } from "../api";
import { DocumentsActivityFold } from "./DocumentsActivityFold";
import { changedSince } from "./happening";
import { InFlight } from "./InFlight";
import { Lately } from "./Lately";

// ActivityFold is a space's activity folded to one line over its list
// (how many in flight, how many changes this week, the latest), open for
// In flight and Changed lately side by side; nothing, when nothing's
// happening in it.
export function ActivityFold({ repo, activity, open, setOpen }: { repo: string; activity: SpaceActivity; open: boolean; setOpen: (open: boolean) => void }) {
  const lately = activity.lately.map((c) => ({ ...c, repo }));
  const flights = activity.inFlight.map((f) => ({ ...f, repo }));
  if (!lately.length && !flights.length) return null;
  const week = changedSince(activity.lately, Date.now() - 7 * 24 * 3600_000);
  const latest = activity.lately[0];
  return (
    <DocumentsActivityFold inFlight={flights.length} week={week} latest={latest && { title: latest.docs[0]?.title || latest.subject, by: latest.by, at: latest.at }} open={open} setOpen={setOpen}>
      <InFlight flights={flights} flat />
      <Lately lately={lately} flat />
    </DocumentsActivityFold>
  );
}
