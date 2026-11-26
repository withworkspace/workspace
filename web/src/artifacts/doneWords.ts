import type { ArtifactDone } from "../api";

// An artifact's lifetime, in the web UI: Promote, with the version chosen,
// what it went into, and, the first time, where it's to live (a folder in
// a repo, for everyone with it, or this machine only); Discard, asking
// why; Restore; and Clear expired on the list. For one that lives in a
// repo, promoting, discarding and restoring it are each a commit landed on
// that repo's default branch, so each says so before it's done.

// doneWords says what promoting, discarding or restoring did, for a toast.
export function doneWords(what: string, d: ArtifactDone): string {
  let s = `${what} ${d.artifact.id}`;
  if (d.into) s += `, into ${d.into}`;
  if (d.home) s += what === "Discarded" ? `, and removed from ${d.home}` : what === "Restored" ? `, and restored to ${d.home}` : `, and put v${d.artifact.promoted?.version} in ${d.home}`;
  if (d.landed) s += `: ${d.landed}`;
  if (d.started) s += `. Mission ${d.started} needs a brief.`;
  return s;
}
