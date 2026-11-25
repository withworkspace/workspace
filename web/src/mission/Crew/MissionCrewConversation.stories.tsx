import type { Meta, StoryObj } from "@storybook/react-vite";
import type { ReactNode } from "react";
import { serve } from "../../stories/api";
import { storyWorkspace } from "../../stories/missions";
import type { Crew, Transcript } from "../../api";
import { useWorkspace } from "../../workspace/store";
import { CrewConversationView } from "./CrewConversationView";
import { useFileLink } from "./useFileLink";
import { asking, crewAt, editing, latest, none, nothingYet, runningTests, session, slug, toRun } from "./transcripts";

// A crew's page, its Conversation tab: crew as a conversation. Its
// session as it's gone (ws's brief to it, what it said, its steps, an
// interruption, its plan pinned on top) with a box to resume it; at work,
// a step not back yet; stopped on a permission (an edit, a command, or
// only what its screen says) or a question; over ACP; with only its latest
// entries; with no transcript, or nothing in it yet.

const meta: Meta = { title: "Mission/MissionCrewConversation" };
export default meta;

storyWorkspace();
serve(`/api/missions/${slug}/crew/c2/prompt`, {
  title: "Do you want to make this edit to greet_test.go?",
  context: ["greet/greet_test.go", "+ func TestToNobody(t *testing.T) {", "Tip: press shift+tab to allow edits from now on"],
  options: ["Yes", "Yes, and don't ask again", "No"],
});

const Frame = ({ children }: { children: ReactNode }) => <div style={{ maxWidth: 1040, padding: "24px 32px" }}>{children}</div>;
const noop = () => {};

// Drawn is crew's conversation as its page draws it, given the transcript
// rather than following it.
function Drawn({ crew, live, t, onTerminal }: { slug: string; id: string; crew: Crew; live: boolean; t: Transcript; onTerminal?: () => void }) {
  const workspace = useWorkspace();
  const fileLink = useFileLink(workspace, slug, crew);
  return <CrewConversationView slug={slug} id="c2" crew={crew} live={live} t={t} root={workspace.root} fileLink={fileLink} onTerminal={onTerminal} />;
}

export const Ended: StoryObj = {
  render: () => (
    <Frame>
      <Drawn slug={slug} id="c2" crew={crewAt(undefined, { ended: true })} live={false} t={session} />
    </Frame>
  ),
};

export const Latest: StoryObj = {
  render: () => (
    <Frame>
      <Drawn slug={slug} id="c2" crew={crewAt("waiting")} live t={latest} onTerminal={noop} />
    </Frame>
  ),
};

export const Working: StoryObj = {
  render: () => (
    <Frame>
      <Drawn slug={slug} id="c2" crew={crewAt("working")} live t={runningTests} onTerminal={noop} />
    </Frame>
  ),
};

export const WorkingOnAFile: StoryObj = {
  render: () => (
    <Frame>
      <Drawn slug={slug} id="c2" crew={crewAt("working")} live t={editing} onTerminal={noop} />
    </Frame>
  ),
};

export const AsksToEdit: StoryObj = {
  render: () => (
    <Frame>
      <Drawn slug={slug} id="c2" crew={crewAt("permission", { notice: "Claude needs your permission to use Edit" })} live t={editing} onTerminal={noop} />
    </Frame>
  ),
};

export const AsksToRun: StoryObj = {
  render: () => (
    <Frame>
      <Drawn slug={slug} id="c2" crew={crewAt("permission", { notice: "Claude needs your permission to use Bash" })} live t={toRun} onTerminal={noop} />
    </Frame>
  ),
};

export const AsksFromItsScreen: StoryObj = {
  render: () => (
    <Frame>
      <Drawn slug={slug} id="c2" crew={crewAt("permission", { notice: "Claude needs your permission" })} live t={session} />
    </Frame>
  ),
};

export const AsksAQuestion: StoryObj = {
  render: () => (
    <Frame>
      <Drawn
        slug={slug}
        id="c2"
        crew={crewAt("question", { notice: "Should main.go pass the flag's value, or read it from the environment?", options: ["The flag's value", "The environment"] })}
        live
        t={session}
        onTerminal={noop}
      />
    </Frame>
  ),
};

export const AsksInItsSteps: StoryObj = {
  render: () => (
    <Frame>
      <Drawn slug={slug} id="c2" crew={crewAt("question")} live t={asking} />
    </Frame>
  ),
};

export const OverACP: StoryObj = {
  render: () => (
    <Frame>
      <Drawn slug={slug} id="c2" crew={crewAt("working", { structured: true, harness: "cursor-acp" })} live t={runningTests} />
    </Frame>
  ),
};

export const NoTranscript: StoryObj = {
  render: () => (
    <Frame>
      <div style={{ display: "grid", gap: 48 }}>
        <Drawn slug={slug} id="c2" crew={crewAt(undefined, { harness: "shell" })} live={false} t={none} />
        <Drawn slug={slug} id="c2" crew={crewAt("waiting", { harness: "shell" })} live t={none} onTerminal={noop} />
      </div>
    </Frame>
  ),
};

export const NothingYet: StoryObj = {
  render: () => (
    <Frame>
      <Drawn slug={slug} id="c2" crew={crewAt("waiting")} live t={nothingYet} onTerminal={noop} />
    </Frame>
  ),
};
