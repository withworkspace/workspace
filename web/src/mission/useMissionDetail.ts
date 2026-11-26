import { useEffect, useRef } from "react";
import { getMission, type MissionDetail, useEvents } from "../api";
import { useLoaded } from "../load";

// useMissionDetail is a mission as the daemon has it, with the capsules'
// PRs as GitHub last said; again whenever the mission changes (GitHub
// answering differently, too). It's the mission's page's, and a page
// under it (a crew's) that draws the mission's bar.
export function useMissionDetail(slug: string) {
  const mission = useLoaded<MissionDetail>(() => getMission(slug), [slug], { keep: true });
  const { value: d, reload: load } = mission;
  // The mission follows its own topic: told at once when its records
  // change, and of its files and capsules at the server's next look. A
  // stream opened again (the page shown again) catches up; the first is
  // what was just loaded.
  const opened = useRef(false);
  useEffect(() => {
    opened.current = false;
  }, [slug]);
  // Where a capsule stands may not have been read when the page first
  // asked (unread): the daemon reads it behind that, and may be done
  // before this stream's open to say so. So the first is read again too.
  const unread = useRef(false);
  unread.current = !!d?.capsules?.some((c) => c.unread);
  useEvents(`mission:${slug}`, {
    hello: () => {
      if (opened.current || unread.current) load();
      opened.current = true;
    },
    changed: load,
  });
  return mission;
}
