import type { MissionDetail, Repo, Workspace } from "../api";
import { useWorkspaceStore } from "../workspace/store";
import { serve } from "./api";

// The missions ws gave (mission/fixtures, which scripts/story-fixtures.mjs
// takes), with the workspace they're of, for a story that draws a part of
// a mission's page as the page does: a name in a mission's words is drawn
// as its own try says it stands, whichever try the mission came from.
const missions = import.meta.glob<MissionDetail>("../mission/fixtures/*/missions/*.json", { eager: true, import: "default" });
const workspaces = import.meta.glob<Workspace>("../mission/fixtures/*/workspace.json", { eager: true, import: "default" });
const repos = import.meta.glob<{ repos: Repo[] }>("../mission/fixtures/*/repos.json", { eager: true, import: "default" });

// TakenMission is a mission as it was taken: the try it's from, and the
// name it was taken under (its slug, or <slug>.<as>).
export interface TakenMission {
  from: string;
  name: string;
  d: MissionDetail;
}

export const takenMissions: TakenMission[] = Object.entries(missions).map(([path, d]) => ({
  from: path.split("/")[3],
  name: path.split("/").pop()!.replace(/\.json$/, ""),
  d,
}));

// storyWorkspace puts the tries' workspaces together in the store, and
// their repos behind /api/repos. (Where the workspace is, and what it's
// called, are the first try's.)
export function storyWorkspace() {
  serve("/api/repos", { repos: Object.values(repos).flatMap((r) => r.repos) });
  const tries = Object.values(workspaces);
  useWorkspaceStore.setState({
    workspace: {
      ...tries[0],
      missions: tries.flatMap((w) => w.missions),
      conversations: tries.flatMap((w) => w.conversations),
      pages: tries.flatMap((w) => w.pages ?? []),
      artifacts: tries.flatMap((w) => w.artifacts ?? []),
      spaces: tries.flatMap((w) => w.spaces ?? []),
    },
  });
}
