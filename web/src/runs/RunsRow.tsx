import { useEffect, useRef, useState } from "react";
import { Link } from "react-router";
import { endRun, type RunDetail } from "../api";
import type { Reload } from "../load";
import { Button } from "../ui/Button";
import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import { humanError } from "../ui/text";
import { Time } from "../ui/Time";
import { toast } from "../ui/toast";
import { useWorkspace } from "../workspace/store";
import { RunCame } from "./RunCame";
import { RunEnds } from "./RunEnds";
import { RunLink } from "./RunLink";
import { RunMark } from "./RunMark";
import { bare, came, commitSaid, forWhom, listed, placeOf } from "./runRows";
import { RunsNeeds } from "./RunsNeeds";
import { RunsPageRow } from "./RunsPageRow";
import { RunsPageSub } from "./RunsPageSub";
import { RunsReports } from "./RunsReports";
import { RunsServiceMore } from "./RunsServiceMore";
import { RunsTaskMore } from "./RunsTaskMore";
import { SaidTo } from "./SaidTo";

// RunsRow is one run on the Runs page: a service with where it answers,
// what it needs and when it ends, or a task with what it came to, the
// reports it kept and the check it was run for; its output opened under
// it, and Stop (or Clear, once it's ended). show is the page's ?show=:
// the run it names is open, and brought into view.
export function RunsRow({ r, up, reload, show }: { r: RunDetail; up: Set<string>; reload: Reload; show: string }) {
  const conversations = useWorkspace((w) => w.conversations);
  const missions = useWorkspace((w) => w.missions);
  const [busy, setBusy] = useState(false);
  // Asked for by the page's address (?show=): open, and brought into view.
  const shown = show !== "" && (show === r.ref || show.split("#")[0] === r.ref);
  const [open, setOpen] = useState(shown);
  const row = useRef<HTMLLIElement>(null);
  useEffect(() => {
    if (shown) {
      setOpen(true);
      row.current?.scrollIntoView({ block: "center" });
    }
  }, [shown]);
  const live = r.state === "running";
  const task = r.type === "task";
  const end = () => {
    setBusy(true);
    endRun(r.ref)
      .then(() => {
        toast(live ? `Stopped ${r.name}` : `Cleared ${r.name}`, { tone: "done" });
        return reload();
      })
      .catch((e: Error) => toast(humanError(e.message), { tone: "danger" }))
      .finally(() => setBusy(false));
  };
  const by = r.by && r.by !== "you" && r.by !== "ws" ? `crew ${r.by}` : r.by;
  // The check of a mission's step a task was run for, going to the mission.
  const checked = r.check && missions.find((x) => x.slug === r.check?.mission);
  return (
    <RunsPageRow
      rowRef={row}
      ended={!live && !task}
      open={open}
      mark={
        task ? (
          r.result === "running" ? (
            <RunMark live up title="Running" />
          ) : (
            <RunMark task={r.result} title={came(r)} />
          )
        ) : (
          <RunMark live={live} up={live && r.up} title={live ? (r.up ? "Up" : "Not up yet") : undefined} />
        )
      }
      name={r.name}
      nameTitle={r.nref ?? r.ref}
      top={
        <>
          {task ? (
            <RunCame result={r.result}>
              {came(r)}
              {!live && r.ended && (
                <span className="dim">
                  , <Time iso={r.ended} as="relative" />
                </span>
              )}
            </RunCame>
          ) : live ? (
            <RunLink href={r.link} title={`${r.up ? "Open it" : "Nothing's listening on its port yet"}${r.link !== r.local ? `: also ${bare(r.local)}` : ""}`}>
              {bare(r.link)}
            </RunLink>
          ) : (
            <span className="dim">
              {r.state === "exited" ? `exited ${r.status ?? 0}` : "gone: ended outside ws"}
              {r.ended && (
                <>
                  , <Time iso={r.ended} as="relative" />
                </>
              )}
            </span>
          )}
          {live && !task && !r.up && <Chip tone="quiet">not up</Chip>}
          {task && (r.n ?? 0) > 1 && (
            <span className="dim" title={`${r.past ?? 0} of its earlier runs are kept behind it`}>
              run {r.n}
            </span>
          )}
        </>
      }
      sub={
        <>
          <RunsPageSub>
            <span>
              In <SaidTo s={placeOf(r, missions)} />
            </span>
            {(live || !task) && (
              <span>
                For <SaidTo s={forWhom(r, missions, conversations)} />
              </span>
            )}
            {task && r.commit && <span title={r.commit}>At {commitSaid(r)}</span>}
            {r.check && (
              <span>
                Check {r.check.n} of step {r.check.step} of <Link to={`/missions/${r.check.mission}`}>{checked?.title || r.check.mission}</Link>
              </span>
            )}
            {task && <RunsReports r={r} />}
            {live && r.owner !== "run" && (r.neededBy?.length ?? 0) > 0 && <span>Needed by {listed(r.neededBy ?? [])}</span>}
            {live && r.link !== r.local && <span className="mono">{bare(r.local)}</span>}
          </RunsPageSub>
          {live && <RunsNeeds r={r} up={up} />}
          {live && r.ends && <RunEnds>Ends {r.ends}</RunEnds>}
        </>
      }
      started={live || !task ? r.started : undefined}
      actions={
        <>
          <Button variant="ghost" size="small" onClick={() => setOpen(!open)} aria-expanded={open} title={open ? "Hide its output" : "Its output, and how it was started"}>
            <Icon name={open ? "chevron-up" : "terminal"} size={11} /> {open ? "Hide" : "Output"}
          </Button>
          <Button
            variant="ghost"
            size="small"
            disabled={busy}
            onClick={end}
            title={live ? (task ? "Stop it now: it's kept as stopped" : "End it now, and run its recipe's stop") : task ? "Clear it away, with its output, its reports and the runs before it" : "Clear it away, its output with it"}
          >
            {busy ? <Spinner size={11} /> : <Icon name={live ? "stop" : "x"} size={11} />} {live ? "Stop" : "Clear"}
          </Button>
        </>
      }
      more={task ? <RunsTaskMore r={r} by={by} show={shown ? show : ""} /> : <RunsServiceMore r={r} by={by} />}
    />
  );
}
