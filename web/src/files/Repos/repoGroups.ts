import type { Repo } from "../../api";

// The workspace's repos as the Files page's column groups them: the
// ungrouped first, then each group in its order (ws.toml's order, then by
// name), hidden groups after. A repo that's a space of documents has
// Documents for its home: it's apart (spaces), folded away at the foot.
export function repoGroups(repos: Repo[]): { groups: [string, Repo[]][]; spaces: Repo[] } {
  const spaces = repos.filter((r) => r.documents);
  const by = new Map<string, Repo[]>();
  for (const r of repos) if (!r.documents) by.set(r.group ?? "", [...(by.get(r.group ?? "") ?? []), r]);
  const groups = [...by].sort(([a, ra], [b, rb]) => (a === "" ? -1 : b === "" ? 1 : Number(!!ra[0].hidden) - Number(!!rb[0].hidden) || (ra[0].groupOrder ?? 0) - (rb[0].groupOrder ?? 0) || a.localeCompare(b)));
  return { groups, spaces };
}
