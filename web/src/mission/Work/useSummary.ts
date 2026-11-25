import { type CapsuleDetail, capsuleAPI, getSummary } from "../../api";
import { useLoaded } from "../../load";

// useSummary is a capsule in brief, read again when it moves on (its head,
// or whether it's all committed), when the daemon has read it again and
// found it changed (seen: the workspace's readsSeen), and for one with
// changes not committed, whenever fresh changes: what's there is still
// being written. The daemon answers as it last read it, and reads it again
// behind the answer. kept asks only what it has: a row doesn't have a
// capsule read, only the capsule that's shown does. Unread, it's nothing
// yet.
export function useSummary(slug: string, c: CapsuleDetail, o: { seen?: string; fresh?: unknown; kept?: boolean }) {
  return useLoaded(
    c.exists ? () => getSummary(capsuleAPI(slug, c.repo, c.name), o.kept).then((s) => (s.unread ? undefined : s)) : null,
    [slug, c.repo, c.name, c.exists, c.head, c.dirty, o.seen, o.kept, c.dirty ? o.fresh : undefined],
    { keep: true },
  );
}
