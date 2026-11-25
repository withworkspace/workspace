import type { CrewDetail } from "../api";
import { Inline } from "../ui/Inline";
import { Status } from "../ui/Status";
import { crewStatus } from "../ui/statusOf";
import { Time } from "../ui/Time";
import { duration } from "../ui/when";
import { CrewBadge } from "./CrewBadge";
import { endedWords } from "./crewEnded";
import { MissionCrewRow } from "./MissionCrewRow";

// CrewRow is a crew in a line: its colour and id, where it stands, its
// harness and capsule, and what it last said of itself. detail says more
// (Log): when it started and ended, and what it was sent to do; side,
// it's in the details' column.
export function CrewRow({ c, slug, detail, side }: { c: CrewDetail; slug: string; detail?: boolean; side?: boolean }) {
  const ended = endedWords(c);
  return (
    <MissionCrewRow
      id={c.id}
      badge={<CrewBadge id={c.id} live={!c.ended} />}
      status={<Status s={crewStatus(c)} />}
      what={c.role === "briefing" ? "briefing" : c.model ? `${c.harness} on ${c.model}` : c.harness}
      where={c.capsule}
      to={`/missions/${slug}/crew/${c.id}`}
      ended={c.ended}
      side={side}
      note={c.note && <Inline text={c.note} lines={detail ? undefined : 2} />}
      times={
        detail && (
          <>
            Started <Time iso={c.started} />
            {c.endedAt ? (
              <>
                , ended <Time iso={c.endedAt} /> after {duration(c.started, c.endedAt)}
                {ended && <span title={c.exit}>: {ended}</span>}
              </>
            ) : (
              <> · running for {duration(c.started)}{c.status === "done" ? ", session still open" : ""}</>
            )}
          </>
        )
      }
      dispatch={detail && c.dispatch && <Inline text={c.dispatch} lines={3} />}
    />
  );
}
