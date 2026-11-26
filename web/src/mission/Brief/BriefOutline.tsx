import type { MouseEvent, RefObject } from "react";
import type { Stage } from "../../api";
import { Inline } from "../../ui/Inline";
import { editsBySection } from "../briefpage";
import { PlanList } from "../PlanList";
import type { DocId, Reading } from "./docState";
import { flash, stepIn } from "./find";
import { MissionBriefChangesNav } from "./MissionBriefChangesNav";
import { MissionBriefOutline } from "./MissionBriefOutline";
import { MissionBriefOutlineEmpty } from "./MissionBriefOutlineEmpty";
import { MissionBriefOutlineItem } from "./MissionBriefOutlineItem";
import { MissionBriefOutlineList } from "./MissionBriefOutlineList";
import { MissionBriefOutlineSection } from "./MissionBriefOutlineSection";
import { MissionBriefSteps } from "./MissionBriefSteps";
import { useBeingRead } from "./useBeingRead";
import { type Heading, useHeadings } from "./useHeadings";

// BriefOutline is the outline beside a mission's brief and plan: the
// changes still to read (stepped through, or all taken), then the brief's
// sections and the plan's, each with how many changes are to read under
// it, and the plan's steps as the mission's details list them (stages: as
// the plan reads now, so they follow it as it's typed). The section being
// read is marked. docs is the documents' element: their headings are read
// from it as they're drawn (useHeadings).
export function BriefOutline({
  docs,
  briefing,
  brief,
  plan,
  stages,
  planned,
  fresh,
  at,
  move,
  focus,
  acceptAll,
  now,
  request,
}: {
  docs: RefObject<HTMLElement | null>;
  briefing: boolean;
  brief?: Reading;
  plan?: Reading;
  stages: Stage[];
  planned: boolean;
  fresh: boolean; // the plan's new with this draft
  at: number; // which change is in hand, of them all; -1 for none
  move: (by: number) => void;
  focus: (doc: DocId, id: string) => void;
  acceptAll: () => void;
  now?: Stage; // the step the mission's at
  request?: string;
}) {
  const heads = useHeadings(docs);
  const here = useBeingRead(docs, heads);

  const goTo = (el: HTMLElement) => el.scrollIntoView({ block: "start", behavior: "smooth" });
  const waiting = { brief: editsBySection(brief?.base ?? "", brief?.pending ?? []), plan: editsBySection(plan?.base ?? "", plan?.pending ?? []) };
  const toRead = (brief?.pending.length ?? 0) + (plan?.pending.length ?? 0);
  const row = (h: Heading) => {
    const list = waiting[h.doc].get(h.text) ?? [];
    return (
      <MissionBriefOutlineItem key={`${h.doc}:${h.text}`} count={list.length} here={here.has(h.el)} onGo={() => (list.length ? focus(h.doc, list[0].id) : goTo(h.el))}>
        {h.text}
      </MissionBriefOutlineItem>
    );
  };
  // A step of the list goes to it in the plan, which marks it a moment.
  const toStep = (e: MouseEvent) => {
    const n = Number((e.target as Element).closest("[data-plan-step]")?.getAttribute("data-plan-step"));
    const el = n ? stepIn(docs.current, n) : null;
    if (!el) return;
    goTo(el);
    flash(el);
  };
  const of = (doc: DocId) => heads.filter((h) => h.doc === doc);
  return (
    <MissionBriefOutline>
      {toRead > 0 && (
        <MissionBriefOutlineSection
          kind="changes"
          title="To review"
          count={toRead}
          at={
            at >= 0 && (
              <>
                {at + 1} of {toRead}
              </>
            )
          }
        >
          <p>
            {toRead === 1 ? "A change" : `${toRead} changes`} {briefing ? "since previous draft" : "to current version"}
          </p>
          <MissionBriefChangesNav onBefore={() => move(-1)} onNext={() => move(1)} onAcceptAll={acceptAll} />
        </MissionBriefOutlineSection>
      )}
      {!toRead && now && (
        <MissionBriefOutlineSection kind="now" title="Now">
          <p>
            Step {now.n} of {stages.length}: {now.title}
          </p>
        </MissionBriefOutlineSection>
      )}
      <MissionBriefOutlineSection title="Brief">
        {of("brief").length ? <MissionBriefOutlineList>{of("brief").map(row)}</MissionBriefOutlineList> : <MissionBriefOutlineEmpty>No sections yet.</MissionBriefOutlineEmpty>}
      </MissionBriefOutlineSection>
      <MissionBriefOutlineSection title="Plan" fresh={fresh}>
        {planned ? (
          <>
            {of("plan").length > 0 && <MissionBriefOutlineList>{of("plan").map(row)}</MissionBriefOutlineList>}
            {stages.length > 0 && (
              // (Keys go to a step by the plan itself: the list is the pointer's.)
              <MissionBriefSteps onClick={toStep}>
                <PlanList stages={stages} brief />
              </MissionBriefSteps>
            )}
          </>
        ) : (
          <MissionBriefOutlineEmpty>{briefing ? "No plan yet: the planner writes one with the brief." : "No plan: mission works from the brief's stages."}</MissionBriefOutlineEmpty>
        )}
      </MissionBriefOutlineSection>
      {request && (
        <MissionBriefOutlineSection kind="requested" title="Requested">
          <p title={request}>
            <Inline text={request} />
          </p>
        </MissionBriefOutlineSection>
      )}
    </MissionBriefOutline>
  );
}
