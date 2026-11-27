import { scope } from "../keymap/commands";

// The keys on the artifacts' pages: the list (ArtifactsPage), one
// artifact (ArtifactPage), two compared (ComparePage), and one found in a
// repo (FoundArtifactPage). The Keys dialog says those of the page open.

export const artifactsKeys = scope("artifacts", "Artifacts", {
  next: { title: "Move down", keys: ["j", "ArrowDown"], row: "Move down / up", moves: true },
  prev: { title: "Move up", keys: ["k", "ArrowUp"], row: "Move down / up", moves: true },
  open: { title: "Open artifact", keys: ["Enter", "o"] },
});

export const artifactViewKeys = scope("artifacts-artifact", "Artifact", {
  prevVersion: { title: "Previous version", keys: "[", row: "Previous / next version" },
  nextVersion: { title: "Next version", keys: "]", row: "Previous / next version" },
  width: { title: "Next width (fit, 1280, 768, 390)", keys: "w" },
  reload: { title: "Reload", keys: "r" },
  stop: { title: "Stop / run", keys: "x" },
  source: { title: "Toggle source", keys: "s" },
  tweaks: { title: "Toggle tweaks", keys: "t" },
  compare: { title: "Compare with base version", keys: "c" },
  promote: { title: "Promote", keys: "p" },
  newTab: { title: "Open in new tab", keys: "O" },
  back: { title: "Back to Artifacts", keys: "Escape" },
});

export const compareKeys = scope("artifacts-compare", "Compare", {
  rightPrev: { title: "Right (or shown) side: previous version", keys: "[", row: "Right (or shown) side: previous / next version" },
  rightNext: { title: "Right (or shown) side: next version", keys: "]", row: "Right (or shown) side: previous / next version" },
  leftPrev: { title: "Left side: previous version", keys: "{", row: "Left side: previous / next version" },
  leftNext: { title: "Left side: next version", keys: "}", row: "Left side: previous / next version" },
  swap: { title: "Swap sides", keys: "S" },
  diff: { title: "Toggle source diff", keys: "d" },
  other: { title: "Show other side (narrow window)", keys: "t" },
  width: { title: "Next width (fit, 1280, 768, 390)", keys: "w" },
  split: { title: "Unified / split diff", keys: "u" },
  reload: { title: "Reload", keys: "r" },
  stop: { title: "Stop / run", keys: "x" },
  back: { title: "Back", keys: "Escape" },
});

export const foundArtifactKeys = scope("artifacts-found", "Artifact in a repo", {
  width: { title: "Next width (fit, 1280, 768, 390)", keys: "w" },
  reload: { title: "Reload from the repo", keys: "r" },
  newTab: { title: "Open in new tab", keys: "O" },
  back: { title: "Back to Artifacts", keys: "Escape" },
});
