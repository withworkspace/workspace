import type { Meta, StoryObj } from "@storybook/react-vite";
import type { ReactNode } from "react";
import { RefsProvider } from "../../refs/known";
import { storyWorkspace, takenMissions } from "../../stories/missions";
import { Inline } from "../../ui/Inline";
import { MissionCrewBadge } from "../MissionCrewBadge";
import { MissionData } from "../MissionData";
import { Log } from "./Log";
import { MissionLogActor } from "./MissionLogActor";
import { MissionLogCrew, MissionLogHandoff } from "./MissionLogCrew";
import { MissionLogDay, MissionLogEvent, MissionLogEvents } from "./MissionLogEvents";
import { MissionLogMoved } from "./MissionLogMoved";
import { MissionLogTools } from "./MissionLogTools";
import { logFilters } from "./words";
import { Text } from "../../ui/Text";

// A mission's Log tab: as missions ws gave (mission/fixtures) have it, and
// its parts' states drawn from props.

const meta: Meta = { title: "Mission/MissionLog" };
export default meta;

const none = () => {};
const column = (children: ReactNode) => <div style={{ maxWidth: 916, padding: "16px 32px 48px", background: "var(--bg)" }}>{children}</div>;

function Taken({ name }: { name: string }) {
  storyWorkspace();
  const m = takenMissions.find((x) => x.name === name);
  if (!m) return <Text as="p" dim>No mission {name} among the fixtures.</Text>;
  return (
    <RefsProvider>
      <MissionData.Provider value={m.d}>{column(<Log d={m.d} selected={(k) => k === "h:c1"} openArtifact={none} toWork={none} />)}</MissionData.Provider>
    </RefsProvider>
  );
}

// Over days, its crew on Work.
export const Mission: StoryObj = { render: () => <Taken name="export-invoices-as-csv" /> };

// Crew who worked in no capsule of the mission's, under the log, with
// their handoffs (c1's selected).
export const CrewElsewhere: StoryObj = { render: () => <Taken name="checks-unmet" /> };

// Its parts: who did what (each of who), a day's name over them, ws's own
// quiet; the tools over them; a crew under them with its handoff.
export const Parts: StoryObj = {
  render: () =>
    column(
      <>
        <MissionLogTools filters={logFilters} filter="all" onFilter={none} q="" onQ={none} />
        <MissionLogEvents>
          <MissionLogDay>Yesterday</MissionLogDay>
          <MissionLogEvent actor={<MissionLogActor actor="you" who="You" />} who="You" iso="2026-11-24T09:12:00Z" as="clock">
            <Inline text="approved b1" />
          </MissionLogEvent>
          <MissionLogEvent actor={<MissionCrewBadge id="c2" />} iso="2026-11-24T09:20:00Z" as="clock">
            <Inline text="lifted `api/export-csv`" />
          </MissionLogEvent>
          <MissionLogDay>Today</MissionLogDay>
          <MissionLogEvent actor={<MissionLogActor actor="orchestrator" who="Orchestrator" />} who="Orchestrator" iso="2026-11-25T08:00:00Z" as="clock">
            <Inline text="started c3 on step 4: write the docs, saying how the export is laid out and what each column holds" />
          </MissionLogEvent>
          <MissionLogEvent actor={<MissionLogActor actor="agent" who="Agent" />} who="Agent" iso="2026-11-25T08:10:00Z" as="clock">
            <Inline text="proposed the plan" />
          </MissionLogEvent>
          <MissionLogEvent actor={<MissionLogActor actor="ws" who="ws" />} quiet title="exit 129" iso="2026-11-25T08:30:00Z" as="clock">
            <Inline text="c2 ended: its terminal closed" />
          </MissionLogEvent>
        </MissionLogEvents>
        <MissionLogMoved>These worked in none of the mission's capsules.</MissionLogMoved>
        <MissionLogCrew>
          <div>c1, as its row</div>
          <MissionLogHandoff id="h1" onOpen={none}>
            {" Read "}
            c1's handoff
          </MissionLogHandoff>
        </MissionLogCrew>
        <MissionLogCrew>
          <div>c2, as its row</div>
          <MissionLogHandoff id="h2" onOpen={none} selected>
            {" Read "}
            c2's handoff
          </MissionLogHandoff>
        </MissionLogCrew>
      </>,
    ),
};
