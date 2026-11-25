import { useMemo, useState } from "react";
import type { CapsuleDetail, ChangedFile, MissionDetail } from "../../api";
import { fileGroups } from "../workRows";
import { MissionWorkFile } from "./MissionWorkFile";
import { MissionWorkFileGroup } from "./MissionWorkFileGroup";
import { MissionWorkFilter } from "./MissionWorkFilter";
import { MissionWorkHeading } from "./MissionWorkHeading";

// The files that differ from where the capsule left the default branch,
// by top-level folder: a click opens the file in the capsule, with what's
// changed in it marked. Those not committed yet say so; ones to skim are
// quieter. What's typed to filter them, and which folders are shut, is
// kept here.
export function WorkFiles({ d, c, files, more, committed }: { d: MissionDetail; c: CapsuleDetail; files: ChangedFile[]; more?: number; committed: number }) {
  const [filter, setFilter] = useState("");
  const [shut, setShut] = useState<Set<string>>(() => new Set());
  const groups = useMemo(() => fileGroups(files, filter), [files, filter]);
  const toggle = (dir: string) =>
    setShut((cur) => {
      const next = new Set(cur);
      if (!next.delete(dir)) next.add(dir);
      return next;
    });
  return (
    <>
      <MissionWorkHeading count={files.length + (more ?? 0)} end={files.length > 8 ? <MissionWorkFilter value={filter} onChange={setFilter} /> : null}>
        {committed ? "Changed files" : "Not committed"}
      </MissionWorkHeading>
      {groups.map((g) => (
        <MissionWorkFileGroup key={g.dir} dir={g.dir ? `${g.dir}/` : "top level"} n={g.files.length} additions={g.additions} deletions={g.deletions} open={!shut.has(g.dir) || !!filter.trim()} onToggle={() => toggle(g.dir)}>
          {g.files.map((f) => {
            const rest = g.dir ? f.path.slice(g.dir.length + 1) : f.path;
            const i = rest.lastIndexOf("/");
            return (
              <MissionWorkFile
                key={f.path}
                // A file that's gone has nothing to open.
                to={f.status === "deleted" ? undefined : `/missions/${d.slug}/files/${c.repo}/${c.name}?${new URLSearchParams({ path: f.path })}`}
                status={f.status}
                dir={rest.slice(0, i + 1)}
                name={rest.slice(i + 1)}
                additions={f.additions}
                deletions={f.deletions}
                loose={f.uncommitted && committed > 0}
                skim={!!f.skim}
                title={`${f.oldPath ? `${f.oldPath} → ` : ""}${f.path}${f.uncommitted ? " · not committed" : ""}${f.skim ? ` · one to skim: ${f.skim}` : ""}`}
              />
            );
          })}
        </MissionWorkFileGroup>
      ))}
      {!groups.length && <p className="dim">Nothing matches.</p>}
      {!!more && !filter.trim() && <p className="dim">and {more.toLocaleString()} more</p>}
    </>
  );
}
