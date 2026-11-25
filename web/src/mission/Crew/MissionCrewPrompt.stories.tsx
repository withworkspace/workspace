import type { Meta, StoryObj } from "@storybook/react-vite";
import type { ReactNode } from "react";
import { storyWorkspace } from "../../stories/missions";
import { CrewPrompt } from "./CrewPrompt";
import { pendingCall } from "./groups";
import { asking, crewAt, editing, slug, toRun } from "./transcripts";

// What crew are stopped on, as a mission's Summary shows it (compact: no
// diff, no shadow) beside each other: an edit, a command, a question with
// its options, and one asked in its steps.

const meta: Meta = { title: "Mission/MissionCrewPrompt" };
export default meta;

storyWorkspace();

const Frame = ({ children }: { children: ReactNode }) => <div style={{ display: "grid", gap: 24, maxWidth: 720, padding: "24px 32px" }}>{children}</div>;

export const Compact: StoryObj = {
  render: () => (
    <Frame>
      <CrewPrompt slug={slug} crew={crewAt("permission")} pending={pendingCall(editing)} compact />
      <CrewPrompt slug={slug} crew={crewAt("permission")} pending={pendingCall(toRun)} compact />
      <CrewPrompt slug={slug} crew={crewAt("question", { notice: "Should main.go pass the flag's value, or read it from the environment?", options: ["The flag's value", "The environment"] })} compact />
      <CrewPrompt slug={slug} crew={crewAt("question")} pending={pendingCall(asking)} compact />
    </Frame>
  ),
};
