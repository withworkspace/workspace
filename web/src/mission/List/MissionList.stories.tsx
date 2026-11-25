import type { Meta, StoryObj } from "@storybook/react-vite";
import type { Mission, Workspace } from "../../api";
import { RefsProvider } from "../../refs/known";
import { storyWorkspace } from "../../stories/missions";
import { Rows } from "../../ui/Rows";
import { checkingCommand, checkingJudge } from "../samples";
import { MissionListCrew } from "./MissionListCrew";
import { MissionListRow } from "./MissionListRow";
import { MissionRow } from "./MissionRow";
import { Missions } from "./Missions";

// The missions list, as the tries the fixtures were taken from had it
// (each try's workspace: Sorrel's, in every court, and the demo's), and
// its rows' states: selected, flashing, with crew at work, while ws
// checks a step, past.

const meta: Meta = { title: "Mission/MissionList" };
export default meta;

const none = () => {};
const workspaces = import.meta.glob<Workspace>("../fixtures/*/workspace.json", { eager: true, import: "default" });
const of = (name: string) => Object.entries(workspaces).find(([p]) => p.includes(`/${name}/`))![1].missions;

const page = (missions: Mission[]) => {
  storyWorkspace();
  return (
    <RefsProvider>
      <div style={{ background: "var(--bg)" }}>
        <Missions missions={missions} />
      </div>
    </RefsProvider>
  );
};

export const Sorrel: StoryObj = { render: () => page(of("sorrel")) };
export const Demo: StoryObj = { render: () => page(of("demo")) };
export const Nothing: StoryObj = { render: () => page([]) };

// A row each way it can be: selected, flashing (its headline just
// changed), with crew at work, while ws checks a step (a command, then
// the judge), and past.
export const Rows_: StoryObj = {
  name: "Rows",
  render: () => {
    storyWorkspace();
    const ms = of("sorrel");
    const at = ms.find((m) => m.crew.some((c) => !c.ended)) ?? ms[0];
    return (
      <RefsProvider>
        <div style={{ maxWidth: 1000, padding: 16, background: "var(--bg)" }}>
          <Rows role="listbox" aria-label="Rows">
            <MissionRow m={ms[0]} selected flash={false} onPoint={none} onOpen={none} />
            <MissionRow m={ms[1]} selected={false} flash onPoint={none} onOpen={none} />
            <MissionRow m={at} selected={false} flash={false} onPoint={none} onOpen={none} />
            <MissionRow m={{ ...at, headline: { ...at.headline, checking: checkingCommand } }} selected={false} flash={false} onPoint={none} onOpen={none} />
            <MissionRow m={{ ...at, headline: { ...at.headline, checking: checkingJudge } }} selected={false} flash={false} onPoint={none} onOpen={none} />
            <MissionRow m={of("demo").find((m) => m.headline.group === "past")!} past selected={false} flash={false} onPoint={none} onOpen={none} />
          </Rows>
        </div>
      </RefsProvider>
    );
  },
};

// A crew's chip in a row: at it (its icon pulsing, or turning), waiting,
// asking.
export const Crew: StoryObj = {
  render: () => (
    <div style={{ display: "flex", gap: 8, padding: 16, background: "var(--bg)" }}>
      <MissionListCrew id="c1" tone="progress" icon="spinner" live says="Working" />
      <MissionListCrew id="c2" tone="progress" icon="circle-dot" live says="Working" />
      <MissionListCrew id="c3" tone="waiting" icon="clock" says="Waiting" />
      <MissionListCrew id="c4" tone="attention" icon="question" says="Asks you" />
    </div>
  ),
};

// A row with all of it, from props.
export const Row: StoryObj = {
  render: () => (
    <div style={{ maxWidth: 1000, padding: 16, background: "var(--bg)" }}>
      <Rows role="listbox" aria-label="Row">
        <MissionListRow
          id="mission-x"
          to="/missions/x"
          status={{ icon: "spinner", tone: "progress", words: "Crew at work", live: true }}
          title="Paginate the invoices endpoint"
          crew={<MissionListCrew id="c1" tone="progress" icon="spinner" live says="Working" />}
          sub={{ title: "c1 is writing the cursor", said: "c1 is writing the cursor, with a test for the last page" }}
          route={{ words: "Step 2 of 5", title: "Step 2 of 5: Work, crew at work" }}
          capsules={{ words: "api/paginate", title: "api/paginate" }}
          time={{ iso: checkingCommand.since, as: "age" }}
        />
      </Rows>
    </div>
  ),
};
