import type { UpdatesInfo } from "../api";

// standing is where ws stands against its latest release, for the line
// that says so.
export type Standing = "waiting" | "out" | "out-built" | "latest" | "ahead" | "unknown";

export function standingOf(up: Pick<UpdatesInfo, "installed" | "newer" | "release" | "latest" | "version">): Standing {
  if (up.installed) return "waiting";
  if (up.newer) return up.release ? "out" : "out-built";
  if (!up.latest) return "unknown";
  return up.latest === up.version ? "latest" : "ahead";
}
