import type { Meta, StoryObj } from "@storybook/react-vite";
import type { MissionDetail } from "../api";
import { MissionPlan } from "./MissionPlan";
import { MissionPlanChecks } from "./MissionPlanChecks";
import { MissionPlanNote } from "./MissionPlanNote";
import { MissionPlanStep } from "./MissionPlanStep";
import { MissionData, Stepper } from "./parts";

// A mission's plan: steps done, the one it's on and those to come, with
// what's to be known of each (whose it is, its clock, its checks, where
// it runs) and what its checks came to; a step alone in each state; and
// the plans of missions as ws gave them (fixtures/), as Summary draws them.

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
            <MissionPlanChecks
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
          <MissionPlanNote icon="pr" title="ws opens the pull request itself">
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

const missions = import.meta.glob<MissionDetail>("./fixtures/*/missions/*.json", { eager: true, import: "default" });

export const OfMissions: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 28, maxWidth: 560 }}>
      {Object.entries(missions)
        .filter(([, d]) => d.stages?.length)
        .map(([path, d]) => (
          <div key={path}>
            <div className="dim" style={{ marginBottom: 6 }}>
              {d.title}
            </div>
            <MissionData.Provider value={d}>
              <Stepper stages={d.stages} />
            </MissionData.Provider>
          </div>
        ))}
    </div>
  ),
};
