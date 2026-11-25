import type { WhatsNewEntry, WhatsNewInfo } from "../../api";
import { shortVersion } from "../version";

// What What's new says of what it shows, worked out of what ws answered.

// releasesIn is how many releases the entries are from: each entry says
// its release only where there's more than one.
export function releasesIn(entries: WhatsNewEntry[]): number {
  return new Set(entries.map((e) => e.version ?? "")).size;
}

// ledeOf is the line under the title: since which release, and how many
// things across how many releases; for a build of a clone, that it's one.
export function ledeOf(entries: WhatsNewEntry[], info: WhatsNewInfo): string {
  if (!info.release) return "A build of a clone: everything it says is new, with what's in no release yet, as it'll read.";
  const releases = releasesIn(entries);
  const things = entries.length === 1 ? "one thing" : `${entries.length} things`;
  const since = info.seen && info.unseen.length > 0 && entries === info.unseen ? `Since ${shortVersion(info.seen)}` : undefined;
  return [since, entries.length ? `${things}${releases > 1 ? `, across ${releases} releases` : ""}` : undefined].filter(Boolean).join(" · ");
}

// releaseOf is what an entry says of its release.
export const releaseOf = (e: WhatsNewEntry): string => (e.version ? shortVersion(e.version) : "Not released yet");
