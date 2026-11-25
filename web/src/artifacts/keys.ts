// The keys on the artifacts' pages, as the keys' help lists them
// (shell/helpKeys.ts).

export const artifactsKeys: [string, string][] = [
  ["j / k", "Move down and up the list"],
  ["⏎ or o", "Open the artifact"],
];

export const artifactKeys: [string, string][] = [
  ["[ / ]", "The version before, or after"],
  ["w", "The next width: fit, 1280, 768, 390"],
  ["r", "Reload: run it again"],
  ["x", "Stop it, or run it again"],
  ["s", "Its source, file by file, or the frame again"],
  ["t", "Its tweaks, shown or hidden: what its page says can be changed"],
  ["c", "Compare it, side by side: beside the version it's based on"],
  ["p", "Promote it: keep it for good, choosing this version"],
  ["O", "Open it alone, in a tab of its own"],
  ["esc", "Back to Artifacts"],
];

export const compareKeys: [string, string][] = [
  ["[ / ]", "The right one's version before, or after (on a narrow window, the one shown)"],
  ["{ / }", "The left one's version before, or after"],
  ["S", "Swap them: each where the other was"],
  ["d", "Their source diff, or the frames again"],
  ["t", "On a narrow window, the other one"],
  ["w", "The next width: fit, 1280, 768, 390"],
  ["u", "The diff unified, or side by side"],
  ["r", "Reload: run them again"],
  ["x", "Stop them, or run them again"],
  ["esc", "Back"],
];

export const foundArtifactKeys: [string, string][] = [
  ["w", "The next width: fit, 1280, 768, 390"],
  ["r", "Reload: run it again, as its repo has it now"],
  ["O", "Open it alone, in a tab of its own"],
  ["esc", "Back to Artifacts"],
];
