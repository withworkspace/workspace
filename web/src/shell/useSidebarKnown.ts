import { useMemo } from "react";
import type { Plugin, Workspace } from "../api";
import { thingsNamed } from "../refs/known";
import type { Known } from "../refs/refs";

// useSidebarKnown is what the sidebar names by: the workspace's, and how
// the plugins' things are named. (The pages' names are read beside them,
// with each repo's files, which a pin's row doesn't need.)
export function useSidebarKnown(workspace: Workspace | undefined, plugins?: Plugin[]): Known {
  const named = JSON.stringify((plugins ?? []).map((p) => [p.name, p.refs ?? []]));
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const things = useMemo(() => thingsNamed(plugins ?? []), [named]);
  return useMemo(
    () => ({
      repos: new Map(),
      root: workspace?.root,
      missions: workspace?.missions.map((m) => m.slug) ?? [],
      pages: (workspace?.pages ?? []).map((p) => p.id),
      talks: (workspace?.conversations ?? []).map((t) => t.id),
      artifacts: (workspace?.artifacts ?? []).map((a) => a.id),
      org: workspace?.org,
      remotes: workspace?.remotes,
      defaultBranch: workspace?.defaultBranch,
      spaces: (workspace?.spaces ?? []).map((s) => ({ repo: s.repo, folder: s.folder })),
      things,
    }),
    [workspace, things],
  );
}
