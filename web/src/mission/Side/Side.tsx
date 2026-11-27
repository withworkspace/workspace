import { missionKeys } from "../keys";
import type { Artifact, CapsuleDetail, MissionDetail } from "../../api";
import type { Reload } from "../../load";
import { Copyable } from "../../ui/Copyable";
import { Status } from "../../ui/Status";
import { missionStateStatus } from "../../ui/statusOf";
import { Time } from "../../ui/Time";
import { CrewRow } from "../CrewRow";
import { DropButton } from "../Ending/DropButton";
import { EndedCapsules } from "../Ending/EndedCapsules";
import { cameFrom } from "../hrefs";
import { MissionCapsuleBadges } from "../MissionCapsuleBadges";
import { MissionOutput } from "../MissionOutput";
import { MissionSideSection } from "../MissionSideSection";
import { PlanList } from "../PlanList";
import { MissionSide } from "./MissionSide";
import { MissionSideCameFrom } from "./MissionSideCameFrom";
import { MissionSideCapsule } from "./MissionSideCapsule";
import { MissionSideFact, MissionSideFacts } from "./MissionSideFacts";
import { MissionSideMore } from "./MissionSideMore";
import { MissionSideRun } from "./MissionSideRun";
import { ParkButton } from "./ParkButton";
import { SideAbout } from "./SideAbout";
import { Text } from "../../ui/Text";

// How many of its outputs the details list: the rest are on Work.
export const sideArtifacts = 6;

// Side is the mission's details: what it is, where its stages are, its
// documents and capsules, who's working. Beside Summary; over another tab
// (over) it's a card that slides in from the right when open.
export function Side({
  d,
  selected,
  openArtifact,
  review,
  toWork,
  reload,
  updated,
  about,
  over,
  open,
}: {
  d: MissionDetail;
  selected: (key: string) => boolean;
  openArtifact: (a: Artifact) => void;
  review: (c: CapsuleDetail) => void;
  toWork: () => void;
  reload: Reload;
  updated?: string;
  about?: string[];
  over?: boolean;
  open?: boolean;
}) {
  const live = d.crew.filter((c) => !c.ended);
  const accepted = d.stages.filter((s) => s.state === "accepted").length;
  return (
    <MissionSide over={over} open={open}>
      <MissionSideFacts>
        <MissionSideFact label="State" hideLabel kind="state">
          <Status s={missionStateStatus(d.state)} />
          <ParkButton d={d} reload={reload} />
          <DropButton d={d} reload={reload} />
        </MissionSideFact>
        {d.at && (
          <MissionSideFact label="At" kind="at">
            {d.at}
          </MissionSideFact>
        )}
        <MissionSideFact label="Created">
          <Time iso={d.created} />
        </MissionSideFact>
        {updated && (
          <MissionSideFact label="Updated">
            <Time iso={updated} as="relative" />
          </MissionSideFact>
        )}
        <MissionSideFact label="Slug">
          <Copyable text={d.slug} icon="over" />
        </MissionSideFact>
        {about && about.length > 0 && (
          <MissionSideFact label="About" kind="about">
            {about.map((name) => (
              <SideAbout key={name} name={name} />
            ))}
          </MissionSideFact>
        )}
        {d.from && (
          <MissionSideFact label="Raised from" mono>
            {d.from}
          </MissionSideFact>
        )}
        {d.conversation && (
          <MissionSideFact label="Came from">
            <MissionSideCameFrom to={cameFrom(d.conversation)}>{d.conversationTitle || "a conversation"}</MissionSideCameFrom>
          </MissionSideFact>
        )}
      </MissionSideFacts>
      {d.stages.length > 0 && (
        <MissionSideSection title={d.plan ? "Plan" : "Stages"} count={`${accepted} of ${d.stages.length} done`}>
          <PlanList stages={d.stages} reload={reload} />
        </MissionSideSection>
      )}
      {d.artifacts.length > 0 && (
        <MissionSideSection title="Outputs" count={d.artifacts.length}>
          {d.artifacts.slice(0, sideArtifacts).map((a) => {
            const key = `a:${a.ref}`;
            return <MissionOutput key={key} id={`side-${key}`} a={a} compact selected={selected(key)} open={openArtifact} />;
          })}
          {d.artifacts.length > sideArtifacts && (
            <MissionSideMore onClick={toWork} keys={missionKeys.work}>
              {d.artifacts.length - sideArtifacts} more in Work
            </MissionSideMore>
          )}
        </MissionSideSection>
      )}
      <MissionSideSection title="Capsules" count={d.capsules.length || undefined}>
        {d.capsules.map((c) => {
          const key = `c:${c.repo}/${c.name}`;
          return (
            <MissionSideCapsule
              key={key}
              id={`side-${key}`}
              selected={selected(key)}
              onReview={() => review(c)}
              title={c.exists ? "Review" : "Burned: folder deleted"}
              keys={c.exists ? "v" : undefined}
              gone={!c.exists}
              dirty={c.dirty}
              name={
                <>
                  {c.repo}/{c.name}
                </>
              }
              badges={(!!c.ahead || !!c.behind || c.pr || !c.exists) && <MissionCapsuleBadges c={{ ...c, dirty: false }} />}
            />
          );
        })}
        {!d.capsules.length && !d.endedCapsules?.length && <Text as="div" dim>None yet: crew lift one when they start.</Text>}
        <EndedCapsules list={d.endedCapsules} />
      </MissionSideSection>
      {d.runs.length > 0 && (
        <MissionSideSection title="Orchestrator" count={d.runs.length}>
          {d.runs.slice(0, 4).map((r) => (
            <MissionSideRun
              key={r.id}
              to={`/missions/${d.slug}/orchestrator/${r.id}`}
              status={
                <Status
                  s={
                    r.error
                      ? { icon: "x-circle", tone: "danger", words: "Failed" }
                      : r.acted
                        ? { icon: "check-circle", tone: "done", words: "Acted" }
                        : { icon: "circle-dashed", tone: "quiet", words: "Did nothing" }
                  }
                />
              }
              what={r.for.replace(/ \(([^)]+)\) \(\1\)$/, " ($1)")}
              iso={r.started}
            />
          ))}
        </MissionSideSection>
      )}
      <MissionSideSection title="Crew" count={live.length ? `${live.length} working` : undefined}>
        {live.map((c) => (
          <CrewRow key={c.id} c={c} slug={d.slug} side />
        ))}
        {d.crew.length > live.length && (
          <MissionSideMore onClick={toWork} keys={missionKeys.work}>
            {live.length ? `${d.crew.length - live.length} earlier, in Work` : `${d.crew.length} so far, none working now: in Work`}
          </MissionSideMore>
        )}
        {!d.crew.length && <Text as="div" dim>Nobody yet.</Text>}
      </MissionSideSection>
    </MissionSide>
  );
}
