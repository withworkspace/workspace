import type { CapsuleDetail, MissionDetail } from "../../api";

// canOpenPR is whether a capsule could have a pull request opened from it:
// a repo that takes them, commits to open, and none open already (once
// GitHub's been asked).
export function canOpenPR(d: Pick<MissionDetail, "github">, c: Pick<CapsuleDetail, "exists" | "direct" | "pr" | "ahead">): boolean {
  return !!(d.github && c.exists && !c.direct && !c.pr && (c.ahead ?? 0) > 0);
}
