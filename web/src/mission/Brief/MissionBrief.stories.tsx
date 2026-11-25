import type { Meta, StoryObj } from "@storybook/react-vite";
import type { CSSProperties, ReactNode } from "react";
import type { TalkDetail } from "../../api";
import { AskField } from "../../ask/AskField";
import { Markdown } from "../../markdown/LazyMarkdown";
import { serve } from "../../stories/api";
import { storyWorkspace, takenMissions } from "../../stories/missions";
import { Button } from "../../ui/Button";
import { Icon } from "../../ui/Icon";
import { Keys } from "../../ui/Keys";
import { Spinner } from "../../ui/Spinner";
import { Turn, TurnSaid } from "../../ui/Turn";
import { MissionData } from "../MissionData";
import { PlanList } from "../PlanList";
import { Brief } from "./Brief";
import { MissionBrief } from "./MissionBrief";
import { MissionBriefActs } from "./MissionBriefActs";
import { MissionBriefAskFoot } from "./MissionBriefAskFoot";
import { MissionBriefBar } from "./MissionBriefBar";
import { MissionBriefChangesNav } from "./MissionBriefChangesNav";
import { MissionBriefDoc } from "./MissionBriefDoc";
import { MissionBriefDocNote } from "./MissionBriefDocNote";
import { MissionBriefDocs } from "./MissionBriefDocs";
import { MissionBriefDocsNote } from "./MissionBriefDocsNote";
import { MissionBriefDocWaiting } from "./MissionBriefDocWaiting";
import { MissionBriefMain } from "./MissionBriefMain";
import { MissionBriefNav } from "./MissionBriefNav";
import { MissionBriefOutline } from "./MissionBriefOutline";
import { MissionBriefOutlineEmpty } from "./MissionBriefOutlineEmpty";
import { MissionBriefOutlineItem } from "./MissionBriefOutlineItem";
import { MissionBriefOutlineList } from "./MissionBriefOutlineList";
import { MissionBriefOutlineSection } from "./MissionBriefOutlineSection";
import { MissionBriefPlanner } from "./MissionBriefPlanner";
import { MissionBriefPlanProblem } from "./MissionBriefPlanProblem";
import { MissionBriefProposed } from "./MissionBriefProposed";
import { MissionBriefQuoted } from "./MissionBriefQuoted";
import { MissionBriefStart } from "./MissionBriefStart";
import { MissionBriefSteps } from "./MissionBriefSteps";
import { MissionBriefTalk } from "./MissionBriefTalk";
import { MissionBriefToolbar } from "./MissionBriefToolbar";
import { MissionBriefUnfold } from "./MissionBriefUnfold";
import { MissionBriefWhat } from "./MissionBriefWhat";

// A mission's Brief tab, from its parts: the whole tab with a draft to
// read (the bar, the outline, the brief and the plan, the planner beside
// them), and the same with the documents side by side and the planner
// folded away; the bar in each standing; the outline's sections and
// items; a document's head and what's said in and among them; the
// planner's pane and what it proposed; and the tab with nothing drafted.
// Then the Brief tabs of missions as ws gave them (fixtures/): one being
// briefed, its planner's second draft to read (taken with its
// conversation); one waiting to be briefed; one under way; one ended.

const meta: Meta = { title: "Mission/MissionBrief" };
export default meta;

const none = () => {};

const brief = `## Goal
Invoices older than seven years are archived, out of the studio's lists but not deleted.

## Context
- Counted in tax years: seven years after the end of the tax year an invoice was issued in.
- Invoices still owed are left alone.

## Acceptance
- An archived invoice isn't in the lists, and can be brought back.
- Nothing is deleted.
`;

const plan = `## Z
- An archived invoice isn't in the lists, and can be brought back.

## Stops early
- None.

## Steps
1. Work: **Archive by tax year.** A job that archives what's due.
2. Review: **Read the change.** Before its pull request.
3. PR: **Open it.**
`;

// The plan's steps, as a mission's details list them: a mission ws gave.
const staged = takenMissions.find((m) => m.name === "export-invoices-as-csv") ?? takenMissions.find((m) => m.d.stages?.length);

// A wide window gives the tab the page under the mission's bar: here, a
// box of that height.
const frame = (children: ReactNode, height = 640) => <div style={{ display: "flex", flexDirection: "column", height, background: "var(--bg)" }}>{children}</div>;

// The widths the tab gives its parts (MissionBrief.module.css), for a part
// drawn out of it.
const widths = { "--brief-outline": "232px", "--brief-planner": "384px", "--brief-measure": "680px" } as CSSProperties;

const field = <AskField place="inline" placeholder="Ask the planner…" onAsk={none} suggestions={["What's still open?"]} />;

function ADraft({ side, folded }: { side?: boolean; folded?: boolean }) {
  return frame(
    <MissionBrief
      planner={!folded}
      bar={
        <MissionBriefBar>
          <MissionBriefWhat icon="pencil" tone="attention" title="Draft 2 of the brief, with its plan">
            Draft 2 <span className="dim">· 2m ago</span>
          </MissionBriefWhat>
          <MissionBriefToolbar slot={none} inset={0} none="Settle a document's changes, and it's yours to edit" />
          <MissionBriefActs>
            <MissionBriefNav onBefore={none} onNext={none}>
              1 of 3
            </MissionBriefNav>
            <Button variant="ghost" type="button">
              Decline
            </Button>
            <Button variant="primary" type="button">
              Confirm and start <Keys k="mod+⏎" />
            </Button>
            {folded && <MissionBriefUnfold waits onUnfold={none} />}
          </MissionBriefActs>
        </MissionBriefBar>
      }
    >
      <Outline />
      <MissionBriefMain>
        <MissionBriefDocs side={side}>
          <MissionBriefDoc
            id="brief"
            title="The brief"
            side={side}
            toRead={<>3 changes to read</>}
            actions={
              <>
                <Button variant="ghost" size="small" type="button">
                  Decline the rest
                </Button>
                <Button size="small" type="button">
                  Accept the rest
                </Button>
              </>
            }
          >
            <div className="prose">
              <Markdown text={brief} />
            </div>
          </MissionBriefDoc>
          <MissionBriefDoc
            id="plan"
            title="The plan"
            side={side}
            actions={
              <>
                <Button variant="ghost" size="small" type="button">
                  <Icon name="pencil" size={12} /> Edit by hand
                </Button>
                <Button size="small" type="button">
                  <Icon name="sparkle" size={12} /> Rewrite from the brief
                </Button>
              </>
            }
          >
            <div className="prose">
              <Markdown text={plan} />
            </div>
          </MissionBriefDoc>
        </MissionBriefDocs>
        {folded && <MissionBriefAskFoot side={side}>{field}</MissionBriefAskFoot>}
      </MissionBriefMain>
      {!folded && <Planner />}
    </MissionBrief>,
  );
}

function Outline() {
  return (
    <MissionBriefOutline>
      <MissionBriefOutlineSection kind="changes" title="To read" count={3} at={<>1 of 3</>}>
        <p>3 changes since the draft before</p>
        <MissionBriefChangesNav onBefore={none} onNext={none} onAcceptAll={none} />
      </MissionBriefOutlineSection>
      <MissionBriefOutlineSection title="The brief">
        <MissionBriefOutlineList>
          <MissionBriefOutlineItem onGo={none}>Goal</MissionBriefOutlineItem>
          <MissionBriefOutlineItem onGo={none} count={2} here>
            Context
          </MissionBriefOutlineItem>
          <MissionBriefOutlineItem onGo={none} count={1}>
            Acceptance
          </MissionBriefOutlineItem>
        </MissionBriefOutlineList>
      </MissionBriefOutlineSection>
      <MissionBriefOutlineSection title="The plan" fresh>
        <MissionBriefOutlineList>
          <MissionBriefOutlineItem onGo={none}>Z</MissionBriefOutlineItem>
          <MissionBriefOutlineItem onGo={none}>Stops early</MissionBriefOutlineItem>
          <MissionBriefOutlineItem onGo={none}>Steps</MissionBriefOutlineItem>
        </MissionBriefOutlineList>
        {staged && (
          <MissionBriefSteps onClick={none}>
            <MissionData.Provider value={staged.d}>
              <PlanList stages={staged.d.stages} brief />
            </MissionData.Provider>
          </MissionBriefSteps>
        )}
      </MissionBriefOutlineSection>
      <MissionBriefOutlineSection kind="requested" title="Requested">
        <p>Archive invoices older than seven years</p>
      </MissionBriefOutlineSection>
    </MissionBriefOutline>
  );
}

function Planner({ centre }: { centre?: boolean }) {
  return (
    <MissionBriefPlanner
      label="The planner's conversation"
      centre={centre}
      talk={{ id: "t1", to: "/missions/archive-invoices-older-than-seven-years/conversations/t1" }}
      action={
        centre && (
          <Button variant="ghost" size="small" type="button">
            <Icon name="pencil" size={12} /> Write it yourself
          </Button>
        )
      }
      onFold={centre ? undefined : none}
      live
      foot={field}
    >
      <Turn by="agent" dense landed>
        <TurnSaid>
          <Markdown text="Three things before I write it: by issue date or due date? What of invoices still owed? And is archiving deleting?" />
        </TurnSaid>
      </Turn>
      <Turn by="you" dense landed>
        <TurnSaid written>Issue date, yes. Leave the ones still owed alone. No deleting.</TurnSaid>
      </Turn>
      <Turn by="agent" dense landed>
        <TurnSaid>
          <Markdown text="Here's the brief." />
        </TurnSaid>
        <MissionBriefProposed icon="file" name="Draft 1" fate="Replaced by draft 2" struck />
      </Turn>
      <Turn by="you" dense landed>
        <TurnSaid written>
          <MissionBriefQuoted>Invoices older than seven years are archived, out of the studio's lists but not deleted.</MissionBriefQuoted>
          Count it in tax years. Then write the plan.
        </TurnSaid>
      </Turn>
      <Turn by="agent" dense landed>
        <TurnSaid>
          <Markdown text="Counted in tax years now, and here's its plan." />
        </TurnSaid>
        <MissionBriefProposed icon="file" name="Draft 2" fate="What the documents show" shown onGo={none} />
        <MissionBriefProposed icon="list-ordered" name="The plan" fate="With draft 2" />
      </Turn>
    </MissionBriefPlanner>
  );
}

export const Tab: StoryObj = { render: () => <ADraft /> };

export const SideBySideAndFolded: StoryObj = { render: () => <ADraft side folded /> };

export const Bar: StoryObj = {
  render: () => (
    <div style={{ ...widths, display: "grid", gap: 6, padding: "16px 0", background: "var(--bg)" }}>
      <MissionBriefBar>
        <MissionBriefWhat icon="pencil" tone="attention" title="Draft 2 of the brief, with its plan">
          Draft 2 <span className="dim">· 2m ago</span>
        </MissionBriefWhat>
        <MissionBriefToolbar slot={none} inset={0} none="Settle a document's changes, and it's yours to edit" />
        <MissionBriefActs>
          <MissionBriefNav onBefore={none} onNext={none}>
            3 changes
          </MissionBriefNav>
          <Button variant="ghost" type="button">
            Decline
          </Button>
          <Button variant="primary" type="button">
            Confirm and start <Keys k="mod+⏎" />
          </Button>
        </MissionBriefActs>
      </MissionBriefBar>
      <MissionBriefBar>
        <MissionBriefWhat icon="pencil" tone="attention">
          Your edits, not saved
        </MissionBriefWhat>
        <MissionBriefToolbar slot={none} inset={24} of="The plan" ofTitle="The other's changes are still to read: settled, it's yours to edit too" />
        <MissionBriefActs>
          <Spinner />
          <Button variant="ghost" type="button">
            Discard my edits
          </Button>
          <Button variant="primary" type="button">
            Save <Keys k="mod+⏎" />
          </Button>
          <MissionBriefUnfold waits onUnfold={none} />
        </MissionBriefActs>
      </MissionBriefBar>
      <MissionBriefBar>
        <MissionBriefWhat icon="check-circle" title="Its brief and plan are written: confirming them starts it">
          Written
        </MissionBriefWhat>
        <MissionBriefToolbar slot={none} inset={0} none="" />
        <MissionBriefActs>
          <Button variant="primary" type="button">
            Confirm and start <Keys k="mod+⏎" />
          </Button>
          <MissionBriefUnfold onUnfold={none} />
        </MissionBriefActs>
      </MissionBriefBar>
      <MissionBriefBar>
        <MissionBriefWhat icon="circle-dot">
          As written <span className="dim">· at step 2 of 5</span>
        </MissionBriefWhat>
        <MissionBriefToolbar slot={none} inset={0} none="" />
        <MissionBriefActs>{null}</MissionBriefActs>
      </MissionBriefBar>
      <MissionBriefBar>
        <MissionBriefWhat icon="check-circle">Ended: as it was written</MissionBriefWhat>
        <MissionBriefToolbar slot={none} inset={0} none="It's ended: nothing to edit" />
        <MissionBriefActs>{null}</MissionBriefActs>
      </MissionBriefBar>
      <MissionBriefBar>
        <MissionBriefWhat icon="file">To brief</MissionBriefWhat>
        <MissionBriefToolbar slot={none} inset={0} none="" />
        <MissionBriefActs>{null}</MissionBriefActs>
      </MissionBriefBar>
    </div>
  ),
};

// (The outline is a wide window's: its pane isn't drawn on a narrow one.)
export const OutlineSections: StoryObj = {
  render: () => (
    <div style={{ display: "grid", gridTemplateColumns: "232px 232px 232px", gap: 24, alignItems: "start", padding: 16, background: "var(--bg)" }}>
      <Outline />
      <MissionBriefOutline>
        <MissionBriefOutlineSection kind="changes" title="To read" count={1}>
          <p>A change to what's written</p>
          <MissionBriefChangesNav onBefore={none} onNext={none} onAcceptAll={none} />
        </MissionBriefOutlineSection>
        <MissionBriefOutlineSection title="The brief">
          <MissionBriefOutlineEmpty>No sections yet.</MissionBriefOutlineEmpty>
        </MissionBriefOutlineSection>
        <MissionBriefOutlineSection title="The plan">
          <MissionBriefOutlineEmpty>No plan yet: its planner writes one with the brief.</MissionBriefOutlineEmpty>
        </MissionBriefOutlineSection>
      </MissionBriefOutline>
      <MissionBriefOutline>
        <MissionBriefOutlineSection kind="now" title="Now">
          <p>Step 2 of 5: Read the change</p>
        </MissionBriefOutlineSection>
        <MissionBriefOutlineSection title="The brief">
          <MissionBriefOutlineList>
            <MissionBriefOutlineItem onGo={none} here>
              Goal
            </MissionBriefOutlineItem>
            <MissionBriefOutlineItem onGo={none}>A heading long enough that it has to wrap onto a second line of the outline</MissionBriefOutlineItem>
          </MissionBriefOutlineList>
        </MissionBriefOutlineSection>
        <MissionBriefOutlineSection kind="requested" title="Requested">
          <p>Archive invoices older than seven years, counted in tax years, leaving those still owed alone, and without deleting anything: a studio should be able to bring one back. Say so in the docs, and in the changelog, and tell the studios who have asked for it that it's there, with what it does to their lists.</p>
        </MissionBriefOutlineSection>
      </MissionBriefOutline>
    </div>
  ),
};

export const Documents: StoryObj = {
  render: () => (
    <div style={{ ...widths, background: "var(--bg)" }}>
      <MissionBriefMain>
        <MissionBriefDocs>
          <MissionBriefDocsNote tone="attention" title="Proposed against an earlier version">
            It's changed since, so accepting this is refused. Ask the planner for it again, from what's written now.
          </MissionBriefDocsNote>
          <MissionBriefDoc id="brief" title="The brief" toRead={<>1 change to read</>}>
            <MissionBriefDocNote>A change couldn't be shown in place: it's in as proposed.</MissionBriefDocNote>
            <div className="prose">
              <Markdown text={brief} />
            </div>
          </MissionBriefDoc>
          <MissionBriefDocsNote tone="attention" title="The plan doesn't read, so the mission works from its brief">
            step 2: no kind
          </MissionBriefDocsNote>
          <MissionBriefDoc
            id="plan"
            title="The plan"
            actions={
              <Button size="small" type="button" disabled aria-busy>
                <Spinner size={12} /> Write it from the brief
              </Button>
            }
          >
            <MissionBriefDocWaiting>No plan yet: its planner writes one with the brief, or from a brief of yours.</MissionBriefDocWaiting>
          </MissionBriefDoc>
          <MissionBriefDocsNote tone="danger" title="ws didn't save the plan">
            <MissionBriefPlanProblem>{"step 2: no kind\nstep 3: a Wait needs who it waits on"}</MissionBriefPlanProblem>
          </MissionBriefDocsNote>
        </MissionBriefDocs>
      </MissionBriefMain>
    </div>
  ),
};

export const PlannerPane: StoryObj = {
  render: () => (
    <div style={{ display: "grid", gridTemplateColumns: "384px 384px 384px", gap: 24, alignItems: "start", height: 560, padding: 16, background: "var(--bg)" }}>
      <Planner />
      <MissionBriefPlanner label="The planner" onFold={none} foot={<AskField place="inline" placeholder="What should change in the plan, and why" onAsk={none} />}>
        <p className="dim">It was briefed without a conversation. Say what should change, and a planner reads the mission and revises its plan. The steps behind the mission stay as they are. A change to a checkpoint or Z comes back to you to accept; anything else applies.</p>
      </MissionBriefPlanner>
      <div style={{ display: "grid", gap: 8 }}>
        <MissionBriefProposed icon="file" name="Draft 2" fate="What the documents show" shown onGo={none} />
        <MissionBriefProposed icon="list-ordered" name="The plan" fate="Accepted" />
        <MissionBriefProposed icon="file" name="Draft 1" fate="Replaced by draft 2" struck />
        <MissionBriefProposed icon="list-ordered" name="Plan revision" fate="Declined" struck />
        <MissionBriefPlanner label="The planner's conversation" reading />
      </div>
    </div>
  ),
};

export const NothingDrafted: StoryObj = {
  render: () => (
    <div style={{ display: "grid", gap: 24 }}>
      {frame(
        <MissionBrief>
          <MissionBriefOutline>
            <MissionBriefOutlineSection title="The brief">
              <MissionBriefOutlineEmpty>Not drafted yet. Its planner writes it with you: the goal, the context, what's in and out, and how you'll know it's done.</MissionBriefOutlineEmpty>
            </MissionBriefOutlineSection>
            <MissionBriefOutlineSection title="The plan">
              <MissionBriefOutlineEmpty>Comes with the brief: what Z is, where it may stop early, and its steps.</MissionBriefOutlineEmpty>
            </MissionBriefOutlineSection>
          </MissionBriefOutline>
          <MissionBriefTalk>
            <MissionBriefStart request="Archive invoices older than seven years" onBrief={none} onOwn={none} />
          </MissionBriefTalk>
        </MissionBrief>,
        420,
      )}
      {frame(
        <MissionBrief>
          <MissionBriefOutline>
            <MissionBriefOutlineSection title="The brief">
              <MissionBriefOutlineEmpty>Not drafted yet. Its planner writes it with you: the goal, the context, what's in and out, and how you'll know it's done.</MissionBriefOutlineEmpty>
            </MissionBriefOutlineSection>
            <MissionBriefOutlineSection title="The plan">
              <MissionBriefOutlineEmpty>Comes with the brief: what Z is, where it may stop early, and its steps.</MissionBriefOutlineEmpty>
            </MissionBriefOutlineSection>
          </MissionBriefOutline>
          <MissionBriefTalk>
            <Planner centre />
          </MissionBriefTalk>
        </MissionBrief>,
        520,
      )}
    </div>
  ),
};

// --- missions as ws gave them ---

// A mission's planner's conversation, as it was taken with the mission
// (fixtures/<try>/talks/<the name the mission was taken under>.json).
const talks = import.meta.glob<TalkDetail>("../fixtures/*/talks/*.json", { eager: true, import: "default" });

function Taken({ name, height }: { name: string; height?: number }) {
  storyWorkspace();
  const m = takenMissions.find((x) => x.name === name);
  if (!m) return <p className="dim">No mission {name} among the fixtures.</p>;
  const talk = Object.entries(talks).find(([path]) => path.endsWith(`/${m.from}/talks/${name}.json`))?.[1];
  if (talk && m.d.planner) serve(`/api/missions/${encodeURIComponent(m.d.slug)}/conversations/${encodeURIComponent(m.d.planner)}`, talk);
  return frame(
    <MissionData.Provider value={m.d}>
      <Brief d={m.d} root={`/try/${m.from}`} reload={async () => {}} onEscape={none} />
    </MissionData.Provider>,
    height,
  );
}

// Being briefed: its planner has proposed the brief twice, the second
// time with its plan (the briefing seed).
export const Briefed: StoryObj = { render: () => <Taken name="archive-invoices-older-than-seven-years.briefed" height={760} /> };

// Waiting to be briefed, with no planner yet.
export const ToBrief: StoryObj = { render: () => <Taken name="archive-invoices-older-than-seven-years" height={480} /> };

// Under way: as written, where the mission is marked on its plan, and a
// planner to ask for a change (it was briefed without a conversation).
export const UnderWay: StoryObj = { render: () => <Taken name="export-invoices-as-csv" height={760} /> };

// Parked, with a brief and no plan.
export const NoPlan: StoryObj = { render: () => <Taken name="invoices-in-more-than-one-currency" /> };

// Ended: as it was written, to read.
export const Ended: StoryObj = { render: () => <Taken name="which-index-is-missing" /> };
