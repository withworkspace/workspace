import type { Meta, StoryObj } from "@storybook/react-vite";
import { type ReactNode, useRef } from "react";
import { Button } from "../../ui/Button";
import { Composer } from "../../ui/Composer";
import { Icon } from "../../ui/Icon";
import { crewStatus } from "../../ui/statusOf";
import { Todos } from "../../ui/Todos";
import { CrewBadge } from "../CrewBadge";
import { latestPlan } from "./groups";
import { MissionCrewDock } from "./MissionCrewDock";
import { MissionCrewHead } from "./MissionCrewHead";
import { Segment, Segmented } from "../../ui/Segmented";
import { MissionCrewPlan } from "./MissionCrewPlan";
import { MissionCrewScreen } from "./MissionCrewScreen";
import { MissionCrewTerminal } from "./MissionCrewTerminal";
import { crewAt, session } from "./transcripts";

// A crew's page around its conversation: its bar as crew stands (at
// work in its capsule, its terminal titled; a briefing agent; over ACP;
// not running; too narrow for all it says), its terminal's frame live and ended (the screen in it is
// xterm's, drawn when the page opens it: here, empty, with what wasn't
// sent), the pill over the dock when there's newer below, and its plan
// folded to its line.

const meta: Meta = { title: "Mission/MissionCrewPage" };
export default meta;

const stop = (
  <Button>
    <Icon name="stop" size={12} /> Stop
  </Button>
);
const notRunning = { icon: "circle", tone: "quiet", words: "Not running" } as const;
const view = (
  <Segmented size="small" value="conversation" onChange={() => {}} aria-label="Crew">
    <Segment value="conversation" keys="t">
      <Icon name="message" size={13} /> Conversation
    </Segment>
    <Segment value="terminal" keys="t">
      <Icon name="terminal" size={13} /> Terminal
    </Segment>
  </Segmented>
);

// What the page puts them in: room as wide as its column.
function MissionCrewPage({ children }: { children: ReactNode }) {
  return <div style={{ display: "grid", gap: 16, maxWidth: 1040 }}>{children}</div>;
}

export const Heads: StoryObj = {
  render: () => (
    <MissionCrewPage>
      <MissionCrewHead name={<CrewBadge id="c2" live />} status={crewStatus(crewAt("working", { status: undefined }))} runs="claude on sonnet" capsule="demo/work-across-repos" termTitle="✳ Greet by name" view={view} actions={stop} />
      <MissionCrewHead name="Briefing agent b1" status={crewStatus(crewAt("waiting", { id: "b1", role: "briefing", status: undefined }))} runs="briefing" view={view} actions={stop} />
      <MissionCrewHead name={<CrewBadge id="c2" live />} status={crewStatus(crewAt("question", { structured: true, status: undefined }))} runs="cursor-acp" acp capsule="demo/work-across-repos" actions={stop} />
      <MissionCrewHead name={<CrewBadge id="c2" />} status={notRunning} runs="claude on sonnet" capsule="demo/work-across-repos" view={view} />
      <div style={{ maxWidth: 560 }}>
        <MissionCrewHead name={<CrewBadge id="c2" live />} status={crewStatus(crewAt("working", { status: undefined }))} runs="claude on sonnet" capsule="demo/work-across-repos-and-more" termTitle="✳ Greet by name in every language" view={view} actions={stop} />
      </div>
    </MissionCrewPage>
  ),
};

function Screen({ error }: { error?: string }) {
  const host = useRef<HTMLDivElement>(null);
  return <MissionCrewScreen ref={host} error={error} />;
}

export const Terminal: StoryObj = {
  render: () => (
    <MissionCrewPage>
      <div style={{ display: "grid", gap: 24 }}>
        <MissionCrewTerminal id="c2" state="live">
          <Screen />
        </MissionCrewTerminal>
        <MissionCrewTerminal id="c2" state="live">
          <Screen error="c2 isn't running" />
        </MissionCrewTerminal>
        <MissionCrewTerminal id="c2" state="ended">
          <Screen />
        </MissionCrewTerminal>
        <MissionCrewTerminal id="c2">
          <Screen />
        </MissionCrewTerminal>
      </div>
    </MissionCrewPage>
  ),
};

const plan = latestPlan(session)!;
const none = () => {};

export const NewerAndFolded: StoryObj = {
  render: () => (
    <MissionCrewPage>
      <div style={{ display: "grid", gap: 48, maxWidth: 820 }}>
        <MissionCrewPlan done={3} of={plan.length} now="Saying how in the README" folded>
          <Todos todos={plan} live />
        </MissionCrewPlan>
        <MissionCrewDock onNewer={none}>
          <Composer value="" onChange={none} onSend={none} aria-label="Message c2" placeholder="Tell c2 what's next…" hint="Typed into its terminal and sent, as if you'd typed it there." />
        </MissionCrewDock>
      </div>
    </MissionCrewPage>
  ),
};
