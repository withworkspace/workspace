import { Link } from "react-router";
import type { RunDetail } from "../api";
import { Chip } from "../ui/Chip";
import { bare } from "./runRows";
import { RunsPageSub } from "./RunsPageSub";
import { textClass } from "../ui/Text";

// RunsNeeds is what a run needs: each need's name, the run that meets it
// (or the address outside ws that does), and the path of the run's own
// name it answers at, if it's mounted. up is the refs of what's running:
// a need whose run has ended says so.
export function RunsNeeds({ r, up }: { r: RunDetail; up: Set<string> }) {
  if (!r.needs?.length) return null;
  return (
    <RunsPageSub>
      {r.needs.map((n) =>
        n.folder ? (
          <span key={n.name}>
            Needs {n.name}:{" "}
            <Link to={n.worktree === "ground" ? `/files/${n.repo}` : `/files/${n.repo}/${n.worktree}`} className={textClass("mono")} title={n.env ? `Passed as $${n.env}: ${n.dir}` : n.dir}>
              {n.repo} / {n.worktree}
            </Link>
          </span>
        ) : (
          <span key={n.name}>
            Needs {n.name}:{" "}
            <a href={n.address} target="_blank" rel="noopener noreferrer" className={textClass("mono")} title={n.env ? `Passed as $${n.env}: ${n.address}` : n.address}>
              {n.run ?? bare(n.address)}
            </a>
            {n.mount && r.link !== r.local && (
              <>
                , at{" "}
                <a href={r.link.replace(/\/$/, "") + n.mount} target="_blank" rel="noopener noreferrer" className={textClass("mono")} title={`${n.name} served under this run's address`}>
                  {n.mount}
                </a>
              </>
            )}
            {n.run && !up.has(n.run) && <Chip tone="waiting">not running</Chip>}
          </span>
        ),
      )}
    </RunsPageSub>
  );
}
