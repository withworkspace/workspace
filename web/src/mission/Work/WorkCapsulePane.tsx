import { Link } from "react-router";
import type { CapsuleDetail, CrewDetail, MissionDetail } from "../../api";
import type { Reload } from "../../load";
import { Button, buttonClass } from "../../ui/Button";
import { Chip } from "../../ui/Chip";
import { Copyable } from "../../ui/Copyable";
import { ErrorCard } from "../../ui/ErrorCard";
import { Icon } from "../../ui/Icon";
import { Keys } from "../../ui/Keys";
import { Skeleton } from "../../ui/Skeleton";
import { Time } from "../../ui/Time";
import { CapsuleEnds } from "../Ending";
import { canOpenPR, OpenPR } from "../OpenPR";
import { shortPath } from "../parts";
import { outputKey, type WorkCapsule } from "../workRows";
import { MissionWorkBody } from "./MissionWorkBody";
import { MissionWorkCommits } from "./MissionWorkCommits";
import { MissionWorkCrew } from "./MissionWorkCrew";
import { MissionWorkEmpty } from "./MissionWorkEmpty";
import { MissionWorkHead } from "./MissionWorkHead";
import { MissionWorkHeading } from "./MissionWorkHeading";
import { MissionWorkLines } from "./MissionWorkLines";
import { MissionWorkLoose } from "./MissionWorkLoose";
import { MissionWorkMade } from "./MissionWorkMade";
import { MissionWorkState } from "./MissionWorkState";
import { MissionWorkSteps } from "./MissionWorkSteps";
import { useSummary } from "./useSummary";
import { WorkCrewLine } from "./WorkCrewLine";
import { WorkFiles } from "./WorkFiles";
import { WorkNow } from "./WorkNow";
import { WorkOutputRow } from "./WorkOutputRow";
import { WorkRuns } from "./WorkRuns";
import { WorkStepLine } from "./WorkStepLine";
import { count } from "./words";

// The open capsule: its head (what it is, where it stands, what you can do
// with it), then who's been in it and the steps that ran there, its
// commits, and its changed files beside them.
export function WorkCapsulePane({ d, x, label, select, review, reload, seen }: { d: MissionDetail; x: WorkCapsule; label: (repo: string) => string; select: (key: string) => void; review: (c: CapsuleDetail) => void; reload: Reload; seen?: string }) {
  const { c } = x;
  // Read again as the mission moves, while what's there isn't committed.
  const { value: sum, error } = useSummary(d.slug, c, { seen, fresh: d.events.length });
  const open = !d.ending && d.state !== "archived";
  // Landed or merged, its work is the default branch's: there's nothing of
  // it left to hand on or drop.
  const settled = !!c.landed || !!c.pr?.merged;
  const files = sum?.changed ?? [];
  const loose = files.filter((f) => f.uncommitted).length;
  // A handoff is on its crew's row already.
  const crewIds = new Set(x.crew.map((k) => k.id));
  const made = x.made.filter((a) => !(a.kind === "handoff" && a.by && crewIds.has(a.by)));
  const handoff = (k: CrewDetail) => x.made.find((a) => a.ref === k.handoff);
  const quiet = !files.length && !x.crew.length && !x.steps.some((s) => s.state !== "todo");
  return (
    <>
      <MissionWorkHead
        icon="box"
        name={label(c.repo)}
        slug={x.id}
        tags={
          <>
            {c.branch && c.branch !== c.name && (
              <Chip tone="quiet" title="Its branch">
                <Icon name="branch" size={11} />
                {c.branch}
              </Chip>
            )}
            <MissionWorkState {...x.state} />
          </>
        }
        actions={
          <>
            {open && canOpenPR(d, c) && <OpenPR d={d} c={c} reload={reload} />}
            {c.exists && (
              <Link to={`/missions/${d.slug}/files/${c.repo}/${c.name}`} className={buttonClass({ size: "small" })} title="Browse its files (f)">
                <Icon name="file" size={12} /> Files <Keys k="f" />
              </Link>
            )}
            {c.exists && (!sum || sum.commitCount > 0) && (
              <Button size="small" onClick={() => review(c)} title="Review its changes (v)">
                <Icon name="eye" /> Review <Keys k="v" />
              </Button>
            )}
            {open && !settled && <CapsuleEnds slug={d.slug} c={c} reload={reload} />}
          </>
        }
        meta={
          <>
            {c.landed && (
              <span>
                Landed straight on the default branch <Time iso={c.landed} />, and burned
              </span>
            )}
            {!c.exists && !c.landed && <span>Burned: its worktree is gone</span>}
            {sum &&
              (sum.base ? (
                <>
                  <span>{sum.commitCount ? `${count(sum.commitCount, "commit")} since it left ${sum.baseRef.replace(/^origin\//, "")}` : "No commits yet"}</span>
                  {sum.files > 0 && (
                    <span>
                      {count(sum.files, "file")} <MissionWorkLines additions={sum.additions} deletions={sum.deletions} />
                    </span>
                  )}
                  <MissionWorkLoose loose={loose > 0}>{loose ? `${count(loose, "file")} not committed` : "nothing uncommitted"}</MissionWorkLoose>
                </>
              ) : (
                <span>Can't tell what's changed: {sum.baseRef} isn't fetched here.</span>
              ))}
            {!!c.behind && <span title={`${c.behind} commits behind the default branch`}>{c.behind} behind</span>}
            {c.exists && (
              <Copyable text={c.path} title="Copy its folder">
                {shortPath(c.path)}
              </Copyable>
            )}
          </>
        }
        pr={c.pr ? c.pr.title : undefined}
      />
      <MissionWorkBody files={files.length > 0 && <WorkFiles d={d} c={c} files={files} more={sum?.changedMore} committed={sum?.commitCount ?? 0} />}>
        {x.now && <WorkNow d={d} x={x} review={review} />}
        {error && !sum && <ErrorCard title="Couldn't read what's changed" error={error} />}
        {x.crew.length > 0 && (
          <>
            <MissionWorkHeading count={x.crew.length}>Crew here</MissionWorkHeading>
            <MissionWorkCrew>
              {x.crew.map((k) => (
                <WorkCrewLine key={k.id} d={d} k={k} steps={x.steps.filter((s) => s.crew?.includes(k.id))} handoff={handoff(k)} select={select} />
              ))}
            </MissionWorkCrew>
          </>
        )}
        <WorkRuns slug={d.slug} c={c} open={open} reload={reload} />
        {x.steps.length > 0 && (
          <>
            <MissionWorkHeading count={x.steps.length}>Steps here</MissionWorkHeading>
            <MissionWorkSteps>
              {x.steps.map((s) => (
                <WorkStepLine key={s.n} d={d} x={x} s={s} />
              ))}
            </MissionWorkSteps>
          </>
        )}
        {sum && sum.commits.length > 0 && (
          <>
            <MissionWorkHeading count={sum.commitCount}>Commits</MissionWorkHeading>
            <MissionWorkCommits commits={sum.commits} more={sum.commitCount - sum.commits.length} />
          </>
        )}
        {made.length > 0 && (
          <>
            <MissionWorkHeading count={made.length}>Made here</MissionWorkHeading>
            <MissionWorkMade>
              {made.map((a) => (
                <WorkOutputRow key={a.ref} a={a} selected={false} select={select} open={() => select(outputKey(a))} made />
              ))}
            </MissionWorkMade>
          </>
        )}
        {quiet && c.exists && (
          <MissionWorkEmpty icon="box" title="Nothing's changed here yet">
            {sum?.base ? `It's level with ${sum.baseRef.replace(/^origin\//, "")}` : "It's as it was lifted"}
            {c.head ? ` at ${c.head.slice(0, 7)}` : ""}.{x.steps[0] ? ` Step ${x.steps[0].n}, ${x.steps[0].title}, is the first in it.` : ""}
          </MissionWorkEmpty>
        )}
        {!sum && !error && c.exists && <Skeleton rows={3} />}
      </MissionWorkBody>
    </>
  );
}
