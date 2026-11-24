import type { Transcript } from "../api";
import type { TranscriptOp } from "../wire.gen";

// shownEntries is how many of a session's latest entries a page holds, as
// the server sends them (web.TranscriptShown).
export const shownEntries = 400;

// appliedTranscript is a crew's session as held with a change told over the
// socket applied: undefined when the change is in it already (it's held as
// of as much of the file, or more), and "read" when it can't be applied:
// nothing's held, entries it should have are missing, or what changed is
// before the latest it holds (a call long ago answered). It's read whole then.
export function appliedTranscript(held: Transcript | undefined, op: TranscriptOp): Transcript | undefined | "read" {
  if (!held) return "read";
  if (held.version !== undefined && op.version <= held.version && held.available) return undefined;
  const base = held.earlier ?? 0;
  if (op.from < base || op.from > base + held.entries.length) return "read";
  const entries = [...held.entries.slice(0, op.from - base), ...op.entries];
  const over = Math.max(0, entries.length - shownEntries);
  return {
    harness: op.harness,
    available: op.available,
    why: op.why,
    entries: over ? entries.slice(over) : entries,
    earlier: base + over || undefined,
    version: op.version,
  };
}
