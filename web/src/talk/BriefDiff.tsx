import { changes } from "../annotate/diff";
import { Change } from "../ui/Change";
import { Text } from "../ui/Text";

// BriefDiff is what an edit to the brief changes: each changed passage,
// as it read and as it would.
export function BriefDiff({ was, now, className }: { was: string; now: string; className?: string }) {
  const lines = now.replace(/\r\n/g, "\n").replace(/\n$/, "").split("\n");
  const hunks = changes(was, now);
  if (!hunks.length) return <Text as="div" dim>No change.</Text>;
  return <Change className={className} wrap struck hunks={hunks.map((h) => ({ was: h.old, now: lines.slice(h.newStart - 1, h.newEnd) }))} />;
}
