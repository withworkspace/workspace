import type { Meta, StoryObj } from "@storybook/react-vite";
import { GroupHead } from "../ui/GroupHead";
import { Rows } from "../ui/Rows";
import { OrchestratorFacts } from "./OrchestratorFacts";
import { OrchestratorLede } from "./OrchestratorLede";
import { OrchestratorMissionRow } from "./OrchestratorMissionRow";
import { OrchestratorNone } from "./OrchestratorNone";
import { OrchestratorOff } from "./OrchestratorOff";
import { OrchestratorPrompt } from "./OrchestratorPrompt";
import { OrchestratorRunMeta } from "./OrchestratorRunMeta";
import { OrchestratorRunRow } from "./OrchestratorRunRow";
import { outcome } from "./words";

// The orchestrator's page and a run's, from their parts: how it's set up
// (on, and off with the lines that turn it on), the missions it's on now,
// its runs in every way one goes, and a run's facts over its prompt.

const meta: Meta = { title: "Orchestrator/Orchestrator" };
export default meta;

const ago = (min: number) => new Date(Date.now() - min * 60_000).toISOString();
const frame: React.CSSProperties = { maxWidth: 1120, padding: 24 };

export const Page: StoryObj = {
  render: () => (
    <div style={frame}>
      <OrchestratorLede>
        An agent that decides what happens between crew: when crew report done or go quiet, when you answer it or review, when a PR changes. It reads the mission, then accepts a stage, dispatches the next crew, or asks you.
      </OrchestratorLede>
      <OrchestratorFacts agent="claude · haiku" auto most={3} />
      <section>
        <GroupHead count={2}>On it now</GroupHead>
        <Rows>
          <OrchestratorMissionRow to="/missions/a" title="Rate-limit the public API" status={{ icon: "sparkle", tone: "progress", words: "Deciding" }} headline="Looking at what c2 reported: **done**, with the tests passing." />
          <OrchestratorMissionRow to="/missions/b" title="Export invoices as CSV" status={{ icon: "sparkle", tone: "progress", words: "Deciding" }} headline="c1 went quiet: reading its last turn." />
        </Rows>
      </section>
      <section>
        <GroupHead count={5}>Recent runs</GroupHead>
        <Rows>
          <OrchestratorRunRow to="/r/1" outcome={outcome({ acted: true })} mission="Rate-limit the public API" on="c2 (done)" said="Accepted step 2 and dispatched c3 for the docs." started={ago(3)} />
          <OrchestratorRunRow to="/r/2" outcome={outcome({ acted: true, retry: true })} mission="Export invoices as CSV" on="c1 (quiet)" said="Asked you whether the export should include voided invoices." started={ago(40)} />
          <OrchestratorRunRow to="/r/3" outcome={outcome({ acted: false })} mission="Export invoices as CSV" on="PR #41 (checks)" started={ago(90)} />
          <OrchestratorRunRow to="/r/4" outcome={outcome({ acted: true, refused: 2 })} mission="Show VAT by rate on the PDF" on="your answer" said="Tried to merge the PR, which isn't its to do." started={ago(60 * 26)} />
          <OrchestratorRunRow to="/r/5" outcome={outcome({ acted: false, error: "harness exited 1" })} mission="Show VAT by rate on the PDF" on="c1 (done)" started={ago(60 * 24 * 3)} />
        </Rows>
      </section>
    </div>
  ),
};

// Off, it says so, and how to turn it on; with nothing to list, a line.
export const Off: StoryObj = {
  render: () => (
    <div style={frame}>
      <OrchestratorOff />
      <OrchestratorFacts agent="claude" auto={false} most={1} />
      <section>
        <GroupHead count={0}>On it now</GroupHead>
        <OrchestratorNone>Nothing right now.</OrchestratorNone>
      </section>
    </div>
  ),
};

// A run's page: its facts under the head, and the prompt it was given,
// folded (open it to see it).
export const Run: StoryObj = {
  render: () => (
    <div style={{ ...frame, maxWidth: 820 }}>
      <OrchestratorRunMeta on="c2 (done)" started={ago(3)} took="41s" agent="claude · haiku" />
      <section>
        <GroupHead count={0}>What it ran</GroupHead>
        <OrchestratorNone>No calls recorded: only Claude Code reports them.</OrchestratorNone>
      </section>
      <section>
        <OrchestratorPrompt text={"You're the orchestrator of **Rate-limit the public API**.\n\n- Read the mission: `ws mission show`.\n- Then accept a stage, dispatch the next crew, or ask."} />
      </section>
    </div>
  ),
};
