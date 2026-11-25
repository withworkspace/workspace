import type { Artifact, CrewDetail, MissionDetail, Stage } from "../../api";
import { Icon } from "../../ui/Icon";
import { Inline } from "../../ui/Inline";
import { Status } from "../../ui/Status";
import { crewStatus } from "../../ui/statusOf";
import { Time } from "../../ui/Time";
import { CrewBadge, endedWords } from "../parts";
import { outputKey } from "../workRows";
import { MissionWorkCrewLine } from "./MissionWorkCrewLine";
import { MissionWorkLeft } from "./MissionWorkLeft";
import { howEnded } from "./words";

// A crew who's been in the capsule: where it stands or how it ended, the
// steps it took and when, and what it left.
export function WorkCrewLine({ d, k, steps, handoff, select }: { d: MissionDetail; k: CrewDetail; steps: Stage[]; handoff?: Artifact; select: (key: string) => void }) {
  const ended = endedWords(k);
  return (
    <MissionWorkCrewLine
      badge={<CrewBadge id={k.id} live={!k.ended} />}
      to={`/missions/${d.slug}/crew/${k.id}`}
      linkTitle={k.ended ? `Read ${k.id}'s transcript` : `Open ${k.id}'s terminal`}
      link={
        k.ended ? (
          "Transcript"
        ) : (
          <>
            <Icon name="terminal" size={12} /> Terminal
          </>
        )
      }
      left={
        handoff ? (
          <MissionWorkLeft onOpen={() => select(outputKey(handoff))} title={`Read ${k.id}'s handoff`}>
            <Inline text={handoff.label ?? k.note ?? "Its handoff"} lines={2} />
          </MissionWorkLeft>
        ) : (
          k.note && (
            <MissionWorkLeft>
              <Inline text={k.note} lines={2} />
            </MissionWorkLeft>
          )
        )
      }
    >
      {k.ended ? <strong title={ended}>{howEnded(k)}</strong> : <Status s={crewStatus(k)} />}{" "}
      <span className="dim">
        {steps.map((s) => `step ${s.n}, ${s.title}`).join("; ")}
        {steps.length > 0 && " · "}
        <Time iso={k.started} />
        {k.endedAt ? (
          <>
            {" "}
            to <Time iso={k.endedAt} as="clock" />
          </>
        ) : (
          ", still here"
        )}
      </span>
    </MissionWorkCrewLine>
  );
}
