import type { Meta, StoryObj } from "@storybook/react-vite";
import { useRef } from "react";
import { Button } from "../../ui/Button";
import { Composer } from "../../ui/Composer";
import { Icon } from "../../ui/Icon";
import { crewStatus } from "../../ui/statusOf";
import { Todos } from "../../ui/Todos";
import { CrewBadge } from "../CrewBadge";
import { latestPlan } from "./groups";
import { MissionCrewDock } from "./MissionCrewDock";
import { MissionCrewHead } from "./MissionCrewHead";
import { MissionCrewPage } from "./MissionCrewPage";
import { MissionCrewPlan } from "./MissionCrewPlan";
import { MissionCrewScreen } from "./MissionCrewScreen";
import { MissionCrewTerminal } from "./MissionCrewTerminal";
import { crewAt, session } from "./transcripts";

// A crew's page around its conversation: its head as crew stands (at
// work in its capsule, its terminal titled; a briefing agent; over ACP;
// not running), its terminal's frame live and ended (the screen in it is
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

export const Heads: StoryObj = {
  render: () => (
    <MissionCrewPage>
      <MissionCrewHead name={<CrewBadge id="c2" live large />} status={crewStatus(crewAt("working", { status: undefined }))} runs="claude on sonnet" capsule="demo/work-across-repos" termTitle="✳ Greet by name" actions={stop} />
      <MissionCrewHead name="Briefing agent b1" status={crewStatus(crewAt("waiting", { id: "b1", role: "briefing", status: undefined }))} runs="briefing" actions={stop} />
      <MissionCrewHead name={<CrewBadge id="c2" live large />} status={crewStatus(crewAt("question", { structured: true, status: undefined }))} runs="cursor-acp" acp capsule="demo/work-across-repos" actions={stop} />
      <MissionCrewHead name={<CrewBadge id="c2" large />} status={notRunning} runs="claude on sonnet" capsule="demo/work-across-repos" />
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
