import { annotateKeys } from "../../annotate/Annotate";

// The keys of one of a mission's documents (ArtifactPage.tsx): a
// document's own, to move through it and comment, then the page's.
export const artifactKeys: [string, string][] = [
  ...annotateKeys.map(([k, what]): [string, string] => (k === "j / k" ? [k, `${what} (in code: scroll)`] : [k, what])),
  ["e", "Edit it (a document in a capsule, or the mission's own)"],
  ["mod+D / mod+U", "Down / up half a page"],
  ["g / G", "Top / bottom"],
  ["o", "Open it on GitHub (or where it lives)"],
  ["r", "Reload"],
  ["esc", "Back"],
];
