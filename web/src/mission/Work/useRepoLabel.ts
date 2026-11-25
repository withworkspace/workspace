import { useCallback } from "react";
import { getRepos } from "../../api";
import { useLoaded } from "../../load";

// useRepoLabel is a repo's name as the workspace calls it (ws, for
// workspace), and its own until that's known.
export function useRepoLabel(): (repo: string) => string {
  const { value: repos } = useLoaded(getRepos, [], { keep: true });
  return useCallback((repo: string) => repos?.find((r) => r.name === repo)?.label ?? repo, [repos]);
}
