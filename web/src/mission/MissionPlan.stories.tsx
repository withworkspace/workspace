import type { Meta, StoryObj } from "@storybook/react-vite";
import type { MissionDetail } from "../api";
import { Chip } from "../ui/Chip";
import { MissionCrewBadge } from "./MissionCrewBadge";
import { MissionPlan } from "./MissionPlan";
import { MissionPlanCheckList } from "./MissionPlanCheckList";
import { MissionPlanFold } from "./MissionPlanFold";
import { MissionPlanLine } from "./MissionPlanLine";
import { MissionPlanNote } from "./MissionPlanNote";
import { MissionPlanNow } from "./MissionPlanNow";
import { MissionPlanPip } from "./MissionPlanPip";
import { MissionPlanStep } from "./MissionPlanStep";
import { MissionStepCard } from "./MissionStepCard";
import { MissionData } from "./MissionData";
import { Stepper } from "./Stepper";
import { PlanList } from "./PlanList";
import { Text } from "../ui/Text";

// A mission's plan: steps done, the one it's on and those to come, with
// what's to be known of each (whose it is, its clock, its checks, where
// it runs) and what its checks came to; a step alone in each state; the
// same plan as rows, as a mission's details have it (folds of the steps
// not shown, each a pip; the step it's on opened in place; a step
// skipped); what a row's card says; and the plans of missions as ws gave
// them (fixtures/), as Summary draws them and as the details do.

const meta: Meta = { title: "Mission/MissionPlan" };
export default meta;

export const Plan: StoryObj = {
  render: () => (
    <div className="page" style={{ maxWidth: 520 }}>
      <MissionPlan>
        <MissionPlanStep n={1} title="Limit each key" state="accepted" hint="Work: a limit a key, in the gateway">
          <MissionPlanNote icon="check" title="Round 2: all met">
            3 of 3 met
          </MissionPlanNote>
        </MissionPlanStep>
        <MissionPlanStep n={2} title="Read it" state="accepted">
          <MissionPlanNote icon="user" yours title="A checkpoint: only you pass it">
            yours
          </MissionPlanNote>
        </MissionPlanStep>
        <MissionPlanStep
          n={3}
          title="Say so in the docs"
          state="current"
          why="Its second round came up short."
          under={
            <MissionPlanCheckList
              checks={[
                { text: "The limits page names the header a client reads", result: "unmet", why: "docs/limits.md doesn't mention X-RateLimit-Remaining", kept: "r12" },
                { text: "`make docs` builds", result: "met" },
                { text: "It reads well to someone new", result: "yours" },
              ]}
            />
          }
        >
          <MissionPlanNote icon="clock" title="Its limit: 30 minutes">
            until 14:30
          </MissionPlanNote>
          <MissionPlanNote icon="alert" late title="Round 2">
            1 of 3 met
          </MissionPlanNote>
          <MissionPlanNote icon="rotate" title="Started again once">
            attempt 2
          </MissionPlanNote>
        </MissionPlanStep>
        <MissionPlanStep n={4} title="Open it" state="todo">
          <MissionPlanNote icon="pr" title="Workspace opens the pull request itself">
            ws opens it
          </MissionPlanNote>
        </MissionPlanStep>
        <MissionPlanStep n={5} title="In each service" state="todo">
          <MissionPlanNote icon="layers">in each service</MissionPlanNote>
          <MissionPlanNote icon="question" title="Only if the gateway's limit isn't enough">
            only if
          </MissionPlanNote>
        </MissionPlanStep>
      </MissionPlan>
    </div>
  ),
};

export const Steps: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 18, maxWidth: 520 }}>
      {(["accepted", "current", "todo"] as const).map((state) => (
        <MissionPlan key={state}>
          <MissionPlanStep n={2} title={`A step that's ${state === "accepted" ? "done" : state === "current" ? "the one it's on" : "to come"}`} state={state} />
        </MissionPlan>
      ))}
    </div>
  ),
};

const checks = [
  { text: "The limits page names the header a client reads", result: "unmet", by: "c3" },
  { text: "`make docs` builds", result: "met" },
  { text: "It reads well to someone new", result: "yours" },
  { text: "The changelog has a line for it" },
] as const;

export const Rows: StoryObj = {
  render: () => (
    <div className="page" style={{ width: 300 }}>
      <MissionPlan rows>
        <MissionPlanFold side="earlier" count={3} onToggle={() => {}} note={{ text: "1 passed with a check unmet", tone: "warn" }}>
          <MissionPlanPip state="accepted" />
          <MissionPlanPip state="accepted" warn />
          <MissionPlanPip state="accepted" />
        </MissionPlanFold>
        <MissionPlanStep row n={4} title="Limit each key, in the gateway, by its plan" state="accepted" kind={{ icon: "tool", says: "Work" }} tabIndex={0} />
        <MissionPlanStep row n={5} title="Try it against staging" state="accepted" skipped kind={{ icon: "list-checks", says: "Verify" }} tabIndex={0} />
        <MissionPlanStep
          row
          n={6}
          title="Say so in the docs"
          state="current"
          opened
          tabIndex={0}
          under={
            <MissionPlanNow>
              <MissionPlanLine icon="tool">
                <strong>Work</strong>
              </MissionPlanLine>
              <MissionPlanLine words>Makes a page on limits, and a line in the changelog</MissionPlanLine>
              <MissionPlanLine>
                <MissionCrewBadge id="c3" live />
                <span>working</span>
              </MissionPlanLine>
              <MissionPlanLine icon="rotate">Attempt 2</MissionPlanLine>
              <MissionPlanLine icon="alert" warn>
                Checks, round 2: 1 of 4 met
              </MissionPlanLine>
              <MissionPlanCheckList checks={[...checks]} />
            </MissionPlanNow>
          }
        />
        <MissionPlanStep row n={7} title="Read it" state="todo" yours kind={{ icon: "eye", says: "Review" }} tabIndex={0}>
          <MissionPlanNote icon="user" yours>
            yours
          </MissionPlanNote>
        </MissionPlanStep>
        <MissionPlanStep row n={8} title="Open it" state="todo" kind={{ icon: "pr", says: "Open" }} tabIndex={0} />
        <MissionPlanFold side="later" count={4} onToggle={() => {}} note={{ text: "1 yours", tone: "yours" }}>
          <MissionPlanPip state="todo" />
          <MissionPlanPip state="todo" yours />
          <MissionPlanPip state="todo" wait />
          <MissionPlanPip state="todo" />
        </MissionPlanFold>
      </MissionPlan>
    </div>
  ),
};

export const Folded: StoryObj = {
  render: () => (
    <div className="page" style={{ width: 300 }}>
      <MissionPlan rows>
        <MissionPlanFold side="earlier" count={3} open onToggle={() => {}} />
        <MissionPlanStep row n={1} title="Reproduce it" state="accepted" kind={{ icon: "tool", says: "Work" }} tabIndex={0} />
        <MissionPlanStep row n={2} title="Fix it" state="accepted" kind={{ icon: "tool", says: "Work" }} tabIndex={0} />
        <MissionPlanStep row n={3} title="Read it" state="current" yours kind={{ icon: "eye", says: "Review" }} tabIndex={0}>
          <MissionPlanNote icon="user" yours>
            yours
          </MissionPlanNote>
        </MissionPlanStep>
      </MissionPlan>
    </div>
  ),
};

const card = { display: "grid", gap: 8, width: 340, padding: "10px 12px", border: "1px solid var(--line)", borderRadius: 8, fontSize: "var(--text-sm)" } as const;

export const StepCard: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "flex", gap: 20, alignItems: "start", flexWrap: "wrap" }}>
      <div style={card}>
        <MissionStepCard
          kind={{ icon: "tool", says: "Work" }}
          which="step 6 of 12"
          state="current"
          stands="Now, since 14:02"
          title="Say so in the docs"
          text="A page on limits: what a key gets, the header a client reads, and what happens over it."
          rows={[
            ["Who", <MissionCrewBadge key="c" id="c3" />],
            ["Makes", "a page on limits, and a line in the changelog"],
            ["Limit", "30 minutes"],
          ]}
          checksSay="Checks, round 2: 1 of 4 met"
          checks={<MissionPlanCheckList checks={[...checks]} said />}
        />
      </div>
      <div style={card}>
        <MissionStepCard
          kind={{ icon: "clock", says: "Wait" }}
          which="step 9 of 12"
          state="todo"
          stands="To come"
          title="Until the release is cut"
          rows={[
            ["Waits on", "the platform team"],
            [
              "Passed by",
              <>
                <Chip tone="progress">Watched</Chip> ws looks every ten minutes
              </>,
            ],
          ]}
        />
      </div>
      <div style={card}>
        <MissionStepCard which="stage 1 of 3" state="accepted" stands="Done 09:58" title="Limit each key" rows={[["Took", "46m"]]} />
      </div>
    </div>
  ),
};

const missions = import.meta.glob<MissionDetail>("./fixtures/*/missions/*.json", { eager: true, import: "default" });

export const OfMissions: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 28, maxWidth: 560 }}>
      {Object.entries(missions)
        .filter(([, d]) => d.stages?.length)
        .map(([path, d]) => (
          <div key={path}>
            <Text as="div" dim style={{ marginBottom: 6 }}>
              {d.title}
            </Text>
            <MissionData.Provider value={d}>
              <Stepper stages={d.stages} />
            </MissionData.Provider>
          </div>
        ))}
    </div>
  ),
};

export const InDetails: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 28, width: 300 }}>
      {Object.entries(missions)
        .filter(([, d]) => d.stages?.length)
        .map(([path, d]) => (
          <div key={path}>
            <Text as="div" dim style={{ marginBottom: 6 }}>
              {d.title}
            </Text>
            <MissionData.Provider value={d}>
              <PlanList stages={d.stages} />
            </MissionData.Provider>
          </div>
        ))}
    </div>
  ),
};
