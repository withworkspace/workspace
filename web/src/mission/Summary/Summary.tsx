import { missionKeys } from "../keys";
import { useMemo } from "react";
import type { CapsuleDetail, MissionDetail } from "../../api";
import type { Reload } from "../../load";
import { MoreLink } from "../../ui/MoreLink";
import { Summary as SummaryList, SummaryRow } from "../../ui/Summary";
import { briefGoal } from "../brief";
import { CrewRow } from "../CrewRow";
import { DecisionsSummary } from "../DecisionsSummary";
import { Events } from "../Log/Events";
import { MissionCapsuleBadges } from "../MissionCapsuleBadges";
import { Stepper } from "../Stepper";
import { useRepoLabel } from "../Work/useRepoLabel";
import { workWords } from "../Work/words";
import { work } from "../workRows";
import { Inline } from "../../ui/Inline";
import { MissionSummaryCapsule } from "./MissionSummaryCapsule";
import { MissionSummaryMore } from "./MissionSummaryMore";
import { MissionSummarySaid } from "./MissionSummarySaid";
import { MissionSummaryWork } from "./MissionSummaryWork";
import { stepLine } from "./stepLine";
import { SummaryLead } from "./SummaryLead";
import { SummaryNotices } from "./SummaryNotices";
import { isNotice, saidLately, summaryRecent, summaryRecentWithSide } from "./words";
import { Text } from "../../ui/Text";

type Go = "brief" | "work" | "log" | "decisions";

// Summary is what needs you, then the rest in a line or two. It leads
// with where the mission stands: the step it's on, what's happening and
// what moves it on, and the last word from its crew, as one block. With
// the details showing, stages, crew, capsules and documents are there, and
// the rest keeps to the goal and what's happened (full is without them).
export function Summary({
  d,
  selected,
  review,
  full,
  go,
  reload,
}: {
  d: MissionDetail;
  selected: (key: string) => boolean;
  review: (c: CapsuleDetail) => void;
  full: boolean;
  go: (t: Go) => void;
  reload: Reload;
}) {
  const goal = briefGoal(d.brief);
  const w = useMemo(() => work(d), [d]);
  const label = useRepoLabel();
  const live = d.crew.filter((c) => !c.ended);
  const events = d.events.filter((e) => !isNotice(e.text));
  const recent = full ? summaryRecent : summaryRecentWithSide;
  // The step it's on, and what crew last said: in the lead when there's a
  // headline to lead with, else rows of their own.
  const led = d.headlines.length > 0;
  const step = stepLine(d);
  const said = saidLately(d);
  return (
    <>
      <SummaryLead d={d} reload={reload} />
      <SummaryNotices d={d} />
      <SummaryList>
        {goal && (
          <SummaryRow label="Goal">
            <Inline text={goal} />{" "}
            <MissionSummaryMore onClick={() => go("brief")} keys={missionKeys.brief}>
              Brief
            </MissionSummaryMore>
          </SummaryRow>
        )}
        {!led && !full && step && <SummaryRow label="Now">{step}</SummaryRow>}
        {!led &&
          said.map((c) => (
            <SummaryRow key={c.id} label={`From ${c.id}`}>
              <MissionSummarySaid text={c.lastReply!} />
            </SummaryRow>
          ))}
        {full && d.stages.length > 0 && (
          <SummaryRow label={d.plan ? "Plan" : "Stages"}>
            <Stepper stages={d.stages} />
          </SummaryRow>
        )}
        {full && (
          <SummaryRow label="Work">
            {w.hand.map((x) => {
              const c = x.c;
              const key = `c:${c.repo}/${c.name}`;
              return (
                <MissionSummaryCapsule
                  key={x.key}
                  id={`item-${key}`}
                  selected={selected(key)}
                  name={
                    <>
                      {c.repo}/{c.name}
                    </>
                  }
                  branch={c.branch && c.branch !== c.name ? c.branch : undefined}
                  badges={<MissionCapsuleBadges c={c} />}
                  onReview={c.exists ? () => review(c) : undefined}
                />
              );
            })}
            <MissionSummaryWork afterCards={w.hand.length > 0}>
              {workWords(w, label, d.slug) || "Nothing yet: crew lift a capsule when they start."}{" "}
              {(w.capsules.length > 0 || w.outputs.length > 0) && (
                <MoreLink onClick={() => go("work")} keys={missionKeys.work}>
                  See all in Work
                </MoreLink>
              )}
            </MissionSummaryWork>
          </SummaryRow>
        )}
        {full && live.length > 0 && (
          <SummaryRow label="Crew">
            {live.map((c) => (
              <CrewRow key={c.id} c={c} slug={d.slug} />
            ))}
          </SummaryRow>
        )}
        {d.decisions.some((x) => x.state === "current") && (
          <SummaryRow label="Decided">
            <DecisionsSummary d={d} all={() => go("decisions")} keys={missionKeys.decisions} />
          </SummaryRow>
        )}
        {events.length > 0 && (
          <SummaryRow label="Recently">
            <Events events={events.slice(-recent)} lines={2} />
            {events.length > recent && (
              <MissionSummaryMore onClick={() => go("log")} keys={missionKeys.log}>
                Full log ({events.length})
              </MissionSummaryMore>
            )}
          </SummaryRow>
        )}
        {d.from && (
          <SummaryRow label="Raised from">
            <Text dim>{d.from}</Text>
          </SummaryRow>
        )}
      </SummaryList>
    </>
  );
}
