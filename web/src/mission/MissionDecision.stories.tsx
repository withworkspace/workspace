import type { Meta, StoryObj } from "@storybook/react-vite";
import type { Decision, MissionDetail } from "../api";
import { RefsProvider } from "../refs/known";
import { storyWorkspace, takenMissions } from "../stories/missions";
import { Button } from "../ui/Button";
import { GroupHead } from "../ui/GroupHead";
import { Icon } from "../ui/Icon";
import { DecisionsSummary } from "./DecisionsSummary";
import { DecisionsTab } from "./DecisionsTab";
import type { DecisionEffect } from "./decisions";
import { MissionDecision } from "./MissionDecision";
import { MissionDecisionForm } from "./MissionDecisionForm";
import { MissionDecisionGroup } from "./MissionDecisionGroup";
import { MissionData } from "./parts";

// A mission's decisions: one alone in each state (crew's, yours, one made
// by answering, one a check was judged with, the one an address names,
// replaced, withdrawn, as Summary has it); groups of
// them as the Decisions tab lays them out (yours, then the work's by
// step); the form one's written in, for each thing it does; and the
// decisions of missions as ws gave them (fixtures/), as the tab and
// Summary draw them.

const meta: Meta = { title: "Mission/MissionDecision" };
export default meta;

const at = "2026-11-20T14:10:00Z";
const d = (id: string, by: string, what: string, why: string, more: Partial<Decision> = {}): Decision => ({ id, by, what, why, at, state: "current", ...more }) as Decision;

const iso = d("d3", "you", "Dates in the file are ISO 8601 (2026-03-01), whatever the customer's locale", "The file is read by spreadsheets and by scripts, and a locale's dates sort wrongly in both", { step: 2 });
const disabled = d("d6", "you", "With nothing to export, Export stays in the toolbar, disabled", "A button that comes and goes moves the rest of the toolbar under the pointer", { step: 2 });
const amounts = d("d1", "c1", "Amounts are written as a customer reads them (1094.40), not in minor units", "The proposal asks for a file a customer opens in a spreadsheet, and a spreadsheet would read 109440 as the amount", { step: 1 });
const rows = d("d2", "c1", "The export writes each row as it's read, with no limit on how many", "A CSV is taken whole, and the largest account has under ten thousand invoices", { step: 1 });
const run = d("d4", "c2", "The dashboard is run with `pnpm dev`, against the api on :8080", "The verifier needs both running to see the export work", { step: 2 });
const thread = d("d9", "you", "Key the cache per tenant", "Tokens are issued per tenant", { from: "t2" });
const hidden = d("d5", "c2", "Export is hidden while there's nothing to export", "A button that does nothing is noise in the toolbar", { state: "superseded", supersededBy: "d6", step: 2 });
const named = d("d7", "c2", "The download is named invoices.csv", "It's what the endpoint is called", { state: "withdrawn", withdrawnWhy: "The browser names the download from the response", step: 2 });

const answer = d("d10", "you", "A", "", {
  why: undefined,
  step: 1,
  asked: {
    by: "c1",
    text: "A VERSION file would have to be kept in step with the tag by hand, and the release reads the tag. Options: (A, my recommendation) no VERSION file: the release's version is its tag, and nothing in the repo holds it; (B) add a VERSION file, as check 2 asks. A goes against check 2 as worded, so I need your call. Which one?",
  },
});
const asked = d("d11", "you", "Keep this mission open, and fix it here", "", { why: undefined, asked: { by: "orchestrator", text: "The pull request merged, but the hooks still report crew as starting. End it, or fix that here?" } });
const reckoned: DecisionEffect[] = [
  { step: 1, check: 2, result: "met", text: "the greeting comes from `greeting()` in `main.go`, and a `VERSION` file holds the release's version" },
  { step: 1, check: 3, result: "unmet", text: "the README says where the version is kept" },
  { step: 1, text: "Version the greeting" },
];

const acts = (
  <>
    <Button variant="ghost" size="small">
      Replace
    </Button>
    <Button variant="ghost" size="small">
      Withdraw
    </Button>
  </>
);

function Column({ children, width = 880 }: { children: React.ReactNode; width?: number }) {
  return <div style={{ maxWidth: width, margin: "0 auto", padding: "24px 24px 80px" }}>{children}</div>;
}

export const One: StoryObj = {
  name: "A decision",
  render: () => (
    <Column>
      {(
        [
          ["Crew's", <MissionDecision d={amounts} actions={acts} />],
          ["Yours: a rule beside it, and the step it was made at", <MissionDecision d={iso} yours step={2} actions={acts} />],
          ["From somewhere (a thread, a conversation)", <MissionDecision d={thread} yours />],
          ["Yours by answering: the question over it, cut at two lines", <MissionDecision d={answer} yours step={1} actions={acts} />],
          ["An answer to the orchestrator", <MissionDecision d={asked} yours />],
          ["One a step's checks were judged with, and the step passed on", <MissionDecision d={answer} yours effects={reckoned} />],
          ["The one the address names", <MissionDecision d={rows} marked actions={acts} />],
          ["Replaced", <MissionDecision d={hidden} />],
          ["Withdrawn", <MissionDecision d={named} />],
          ["With no why", <MissionDecision d={{ ...run, why: undefined }} />],
          ["On Summary: yours, with no why, cut at two lines", <MissionDecision d={disabled} yours dense />],
          ["On Summary: an answer, its question in a line", <MissionDecision d={answer} yours dense />],
        ] as [string, React.ReactNode][]
      ).map(([what, part]) => (
        <section key={what}>
          <GroupHead>{what}</GroupHead>
          {part}
        </section>
      ))}
    </Column>
  ),
};

export const Groups: StoryObj = {
  render: () => (
    <Column>
      <MissionDecisionGroup
        title="Yours"
        count={4}
        note="They stand over what the plan asks."
        end={
          <Button variant="ghost" size="small">
            <Icon name="plus" size={12} /> Record a decision
          </Button>
        }
      >
        <MissionDecision d={disabled} yours step={2} actions={acts} />
        <MissionDecision d={iso} yours step={2} actions={acts} />
        <MissionDecisionGroup step title="When you were asked">
          <MissionDecision d={answer} yours step={1} effects={reckoned.slice(0, 1)} actions={acts} />
          <MissionDecision d={asked} yours actions={acts} />
        </MissionDecisionGroup>
      </MissionDecisionGroup>
      <MissionDecisionGroup title="From the work" count={3} note="What crew and the orchestrator settled as they went.">
        <MissionDecisionGroup step title="Step 1 The endpoint">
          <MissionDecision d={amounts} actions={acts} />
          <MissionDecision d={rows} actions={acts} />
        </MissionDecisionGroup>
        <MissionDecisionGroup step title="Step 2 The button">
          <MissionDecision d={run} actions={acts} />
        </MissionDecisionGroup>
      </MissionDecisionGroup>
    </Column>
  ),
};

const nothing = () => {};

export const Forms: StoryObj = {
  render: () => (
    <Column>
      {(
        [
          ["One of your own", <MissionDecisionForm kind="record" onSend={nothing} onCancel={nothing} />],
          ["In place of another: it starts from that one's words, and can't be sent until they change", <MissionDecisionForm kind="replace" of="d1" what={amounts.what} onSend={nothing} onCancel={nothing} />],
          ["Withdrawing one: only why", <MissionDecisionForm kind="withdraw" of="d4" onSend={nothing} onCancel={nothing} />],
          ["Being sent", <MissionDecisionForm kind="record" busy onSend={nothing} onCancel={nothing} />],
          ["Refused", <MissionDecisionForm kind="withdraw" of="d5" error="d5 is superseded already" onSend={nothing} onCancel={nothing} />],
        ] as [string, React.ReactNode][]
      ).map(([what, part]) => (
        <section key={what}>
          <GroupHead>{what}</GroupHead>
          {part}
        </section>
      ))}
    </Column>
  ),
};

// The missions ws gave that have something decided, in the workspace
// they're of.
storyWorkspace();
const decided = takenMissions.filter((t) => t.d.decisions.length > 0).map((t) => ({ name: `${t.from}/${t.name}`, m: t.d }));

function Mission({ m, children }: { m: MissionDetail; children: React.ReactNode }) {
  return (
    <RefsProvider>
      <MissionData.Provider value={m}>{children}</MissionData.Provider>
    </RefsProvider>
  );
}

export const Missions: StoryObj = {
  name: "Missions, on the Decisions tab",
  render: () => (
    <Column>
      {decided.map(({ name, m }) => (
        <section key={name} style={{ marginBottom: 40 }}>
          <GroupHead>
            {m.title} <span className="dim">{name}</span>
          </GroupHead>
          <Mission m={m}>
            <DecisionsTab d={m} mark={nothing} reload={async () => {}} />
          </Mission>
        </section>
      ))}
    </Column>
  ),
};

export const OnSummary: StoryObj = {
  name: "Missions, on Summary",
  render: () => (
    <Column width={640}>
      {decided.map(({ name, m }) => (
        <section key={name} style={{ marginBottom: 32 }}>
          <GroupHead>
            {m.title} <span className="dim">{name}</span>
          </GroupHead>
          <Mission m={m}>
            <DecisionsSummary d={m} all={nothing} />
          </Mission>
        </section>
      ))}
    </Column>
  ),
};
