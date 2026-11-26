// The keys on the artifacts' pages, as the keys' help lists them
// (shell/helpKeys.ts).

export const artifactsKeys: [string, string][] = [
  ["j / k", "Move down / up"],
  ["⏎ or o", "Open artifact"],
];

export const artifactKeys: [string, string][] = [
  ["[ / ]", "Previous / next version"],
  ["w", "Next width (fit, 1280, 768, 390)"],
  ["r", "Reload"],
  ["x", "Stop / run"],
  ["s", "Toggle source"],
  ["t", "Toggle tweaks"],
  ["c", "Compare with base version"],
  ["p", "Promote"],
  ["O", "Open in new tab"],
  ["esc", "Back to Artifacts"],
];

export const compareKeys: [string, string][] = [
  ["[ / ]", "Right (or shown) side: previous / next version"],
  ["{ / }", "Left side: previous / next version"],
  ["S", "Swap sides"],
  ["d", "Toggle source diff"],
  ["t", "Show other side (narrow window)"],
  ["w", "Next width (fit, 1280, 768, 390)"],
  ["u", "Unified / split diff"],
  ["r", "Reload"],
  ["x", "Stop / run"],
  ["esc", "Back"],
];

export const foundArtifactKeys: [string, string][] = [
  ["w", "Next width (fit, 1280, 768, 390)"],
  ["r", "Reload from the repo"],
  ["O", "Open in new tab"],
  ["esc", "Back to Artifacts"],
];
