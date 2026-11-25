import { useMemo } from "react";
import type { Repo } from "../../api";
import { FilesRepoGroup } from "./FilesRepoGroup";
import { FilesRepoRow } from "./FilesRepoRow";
import { FilesRepos } from "./FilesRepos";
import { repoGroups } from "./repoGroups";

// RepoColumn is the workspace's repos, as ws labels them: ungrouped first,
// then each group in its order under its display name, hidden groups
// after, folded (unless what's open is in one). A repo that's a space of
// documents has Documents for its home: it's folded away at the foot, In
// Documents.
export function RepoColumn({ repos, current }: { repos: Repo[]; current: string }) {
  const { groups, spaces } = useMemo(() => repoGroups(repos), [repos]);
  const row = (r: Repo) => <FilesRepoRow key={r.name} to={`/files/${encodeURIComponent(r.name)}`} name={r.name} label={r.label} color={r.color} capsules={r.capsules.length} on={r.name === current} cloned={r.cloned} />;
  const holds = (rs: Repo[]) => rs.some((r) => r.name === current);
  return (
    <FilesRepos count={repos.length}>
      {groups.map(([g, rs]) => (
        <FilesRepoGroup key={rs[0].hidden ? g : g || "-"} kind={rs[0].hidden ? "hidden" : "shown"} label={rs[0].groupLabel || g || undefined} count={rs.length} open={rs[0].hidden ? holds(rs) : undefined}>
          {rs.map(row)}
        </FilesRepoGroup>
      ))}
      {spaces.length > 0 && (
        <FilesRepoGroup kind="documents" count={spaces.length} open={holds(spaces)}>
          {spaces.map(row)}
        </FilesRepoGroup>
      )}
    </FilesRepos>
  );
}
