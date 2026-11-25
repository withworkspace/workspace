import type { Artifact, CapsuleDetail, MissionDetail } from "../../api";
import type { Reload } from "../../load";
import { usePref } from "../../prefs";
import { Empty } from "../../ui/Empty";
import { EndedCapsules } from "../Ending/EndedCapsules";
import { type Work as WorkModel, type WorkCapsule, outputHome, outputKey } from "../workRows";
import { MissionWork } from "./MissionWork";
import { MissionWorkEnded } from "./MissionWorkEnded";
import { MissionWorkGroup } from "./MissionWorkGroup";
import { MissionWorkNone } from "./MissionWorkNone";
import { MissionWorkSub } from "./MissionWorkSub";
import { useRepoLabel } from "./useRepoLabel";
import { WorkCapsulePane } from "./WorkCapsulePane";
import { WorkCapsuleRow } from "./WorkCapsuleRow";
import { WorkOutputPane } from "./WorkOutputPane";
import { WorkOutputRow } from "./WorkOutputRow";
import { doneWords } from "./words";

// The mission's Work tab: what's been made, a row each, and the one that's
// open beside it. Capsules come first, in the order they need you (in
// hand, up next, done, which folds to a line), then its outputs. A capsule
// opens on who's been in it and what they left, the plan's steps that ran
// there, its commits, and the files that differ from where it left the
// default branch; an output opens to be read.
//
// This is what the tab knows and does: which row is open, whether Done is
// folded. What it draws is the parts beside it (MissionWork…), each given
// what to draw; the components named Work… say which part a thing of the
// mission's is drawn as.
export function Work({
  d,
  w,
  shown,
  select,
  review,
  openArtifact,
  reload,
  seen,
}: {
  d: MissionDetail;
  w: WorkModel;
  shown?: string; // the row that's open: a capsule's key or an output's
  select: (key: string) => void;
  review: (c: CapsuleDetail) => void;
  openArtifact: (a: Artifact) => void;
  reload: Reload;
  // seen is the workspace's readsSeen: a capsule's summary read again.
  seen?: string;
}) {
  const label = useRepoLabel();
  const [doneOpen, setDoneOpen] = usePref("workDone");
  const capsule = w.capsules.find((x) => x.key === shown);
  const output = capsule ? undefined : w.outputs.find((a) => outputKey(a) === shown);
  // Done is open while what's shown is in it.
  const showDone = doneOpen || capsule?.where === "done";
  const toggleDone = () => {
    const open = !showDone;
    setDoneOpen(open);
    if (!open && capsule?.where === "done" && w.opensOn && w.opensOn !== capsule.key) select(w.opensOn);
  };
  const rows = (list: WorkCapsule[]) => list.map((x) => <WorkCapsuleRow key={x.key} d={d} x={x} selected={x.key === shown} select={select} label={label} review={review} seen={seen} />);

  if (!w.capsules.length && !w.outputs.length && !d.endedCapsules?.length) {
    return (
      <Empty icon="box" title="Nothing's been made yet">
        A capsule is a worktree the mission's crew work in. Crew lift one when they start; <code>ws lift &lt;repo&gt; &lt;name&gt;</code> lifts one by hand. Proposals, notes, handoffs and
        artifacts show here as crew make them.
      </Empty>
    );
  }
  return (
    <MissionWork
      fills
      list={
        <>
          <MissionWorkGroup count={w.capsules.length}>Capsules</MissionWorkGroup>
          {w.hand.length > 0 && <MissionWorkSub>In hand</MissionWorkSub>}
          {rows(w.hand)}
          {w.next.length > 0 && <MissionWorkSub>Up next</MissionWorkSub>}
          {rows(w.next)}
          {w.done.length > 0 && (
            <MissionWorkSub open={showDone} onToggle={toggleDone} rest={doneWords(w.done, label, d.slug)}>
              Done
            </MissionWorkSub>
          )}
          {showDone && rows(w.done)}
          {!w.capsules.length && <MissionWorkNone>None yet: crew lift one when they start.</MissionWorkNone>}
          {d.endedCapsules?.length ? (
            <MissionWorkEnded>
              <EndedCapsules list={d.endedCapsules} />
            </MissionWorkEnded>
          ) : null}
          <MissionWorkGroup count={w.outputs.length}>Outputs</MissionWorkGroup>
          {w.outputs.map((a) => (
            <WorkOutputRow key={a.ref} a={a} selected={outputKey(a) === shown} select={select} open={openArtifact} />
          ))}
          {!w.outputs.length && <MissionWorkNone>Nothing yet: proposals, notes, handoffs and artifacts show here as crew make them.</MissionWorkNone>}
        </>
      }
    >
      {capsule && <WorkCapsulePane key={capsule.key} d={d} x={capsule} label={label} select={select} review={review} reload={reload} seen={seen} />}
      {output && <WorkOutputPane key={output.ref} d={d} a={output} home={outputHome(d, w, output)} label={label} select={select} open={openArtifact} />}
    </MissionWork>
  );
}
