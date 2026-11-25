// leftNames are what a file left out of what the guide's agent read is
// left out for, counted: "2 lockfiles".
const leftNames: Record<string, [string, string]> = {
  lockfile: ["lockfile", "lockfiles"],
  generated: ["generated", "generated"],
  snapshot: ["snapshot", "snapshots"],
  renamed: ["only renamed", "only renamed"],
  binary: ["binary", "binary"],
  "too large": ["too large", "too large"],
  "no room": ["past the room", "past the room"],
};

// leftWords says what was left out, by why: "2 lockfiles, 1 generated".
export function leftWords(left: { why: string }[]): string {
  const n = new Map<string, number>();
  for (const l of left) n.set(l.why, (n.get(l.why) ?? 0) + 1);
  return [...n].map(([why, k]) => `${k} ${(leftNames[why] ?? [why, why])[k === 1 ? 0 : 1]}`).join(", ");
}
