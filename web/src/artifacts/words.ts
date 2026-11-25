import type { ArtifactFound } from "../api";

// What the artifacts' pages say of things, beside what shown.ts says of an
// artifact of this machine's.

// foundBy says who promoted the version that's in a found artifact's
// folder, where its record says, or who changed it last.
export function foundBy(a: ArtifactFound): string {
  if (a.promoted?.by) return `v${a.promoted.version} promoted by ${a.promoted.by}`;
  return a.changedBy ? `changed by ${a.changedBy}` : "";
}

// diffStatusWords is how a file that isn't only changed is said to stand
// in two versions' source diff.
export const diffStatusWords: Record<string, string> = { added: "added", deleted: "removed", modified: "changed" };
