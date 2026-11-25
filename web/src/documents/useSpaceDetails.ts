import { useEffect, useState } from "react";
import { getSpace, getSpaceActivity, type Space, type SpaceActivity, type SpaceDoc } from "../api";

// What Documents reads of each space beyond its count: its documents and
// what's happening in it, a space's card filled in as it comes; read again
// when ws has found a space other than it said (seen: the workspace's
// spacesSeen). What's happening that's not been read yet isn't shown.
export type SpaceDetail = { docs?: SpaceDoc[]; activity?: SpaceActivity };

export function useSpaceDetails(spaces: Space[], seen?: string): Map<string, SpaceDetail> {
  const [details, setDetails] = useState<Map<string, SpaceDetail>>(new Map());
  const key = `${spaces.map((s) => s.repo).join("\n")}\n${seen ?? ""}`;
  useEffect(() => {
    let stale = false;
    const put = (repo: string, d: SpaceDetail) => !stale && setDetails((m) => new Map(m).set(repo, { ...m.get(repo), ...d }));
    for (const s of spaces) {
      if (!s.cloned) continue;
      getSpace(s.repo).then(
        (r) => put(s.repo, { docs: r.docs }),
        () => {},
      );
      getSpaceActivity(s.repo).then(
        (a) => !a.unread && put(s.repo, { activity: a }),
        () => {},
      );
    }
    return () => {
      stale = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);
  return details;
}
