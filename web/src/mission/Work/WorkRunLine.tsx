import type { RunDetail } from "../../api";
import { RunEnds } from "../../runs/RunEnds";
import { RunLink } from "../../runs/RunLink";
import { RunMark } from "../../runs/RunMark";
import { RunName } from "../../runs/RunName";
import { Button } from "../../ui/Button";
import { Chip } from "../../ui/Chip";
import { Icon } from "../../ui/Icon";
import { Spinner } from "../../ui/Spinner";
import { Time } from "../../ui/Time";
import { MissionWorkRun } from "./MissionWorkRun";
import { WorkTaskLine } from "./WorkTaskLine";

// WorkRunLine is one of a capsule's runs: a service, with where it
// answers while it's running, what it needs and when it ends; or a task
// (WorkTaskLine).
export function WorkRunLine({ r, busy, stop }: { r: RunDetail; busy: boolean; stop: () => void }) {
  const live = r.state === "running";
  if (r.type === "task") return <WorkTaskLine r={r} busy={busy} stop={stop} />;
  return (
    <MissionWorkRun
      mark={<RunMark live={live} up={live && r.up} ended={!live} />}
      under={
        <>
          {live && (r.needs?.length ?? 0) > 0 && (
            <RunEnds>
              Needs{" "}
              {r.needs?.map((n, i) => (
                <span key={n.name}>
                  {i > 0 && ", "}
                  {n.name} (
                  {n.folder ? (
                    // A repo's folder: the worktree that meets it.
                    <span className="mono" title={n.dir}>
                      {n.repo} / {n.worktree}
                    </span>
                  ) : (
                    <a href={n.address} target="_blank" rel="noopener noreferrer" className="mono" title={n.address}>
                      {n.run ?? n.address.replace(/^https?:\/\//, "")}
                    </a>
                  )}
                  {n.mount && r.link !== r.local ? `, at ${n.mount}` : ""})
                </span>
              ))}
            </RunEnds>
          )}
          {live && r.ends && <RunEnds>Ends {r.ends}</RunEnds>}
        </>
      }
      action={
        live && (
          <Button variant="ghost" size="small" disabled={busy} onClick={stop} title="End it now">
            {busy ? <Spinner size={11} /> : <Icon name="stop" size={11} />} Stop
          </Button>
        )
      }
    >
      <RunName title={r.command}>{r.name}</RunName>
      {live ? (
        <RunLink href={r.link} title={`${r.up ? "Open it" : "Nothing's listening on its port yet"}${r.link !== r.local ? `: ${r.local.replace(/^https?:\/\//, "")}` : ""}`}>
          {r.link.replace(/^https?:\/\//, "").replace(/\/$/, "")}
        </RunLink>
      ) : (
        <span className="dim">
          {r.state === "exited" ? `exited ${r.status ?? 0}` : "gone"}
          {r.ended && (
            <>
              , <Time iso={r.ended} as="relative" />
            </>
          )}
        </span>
      )}
      {live && !r.up && <Chip tone="quiet">not up</Chip>}
    </MissionWorkRun>
  );
}
