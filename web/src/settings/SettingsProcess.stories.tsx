import type { Meta, StoryObj } from "@storybook/react-vite";
import { SettingsBlock } from "./SettingsBlock";
import { SettingsProcess } from "./SettingsProcess";
import { SettingsProcessNone } from "./SettingsProcessNone";

// The process on the Settings page: the file as committed, changed since,
// and never committed, over what it says; and what's said where there's
// none yet.

const meta: Meta = { title: "Settings/SettingsProcess" };
export default meta;

const prose = (
  <>
    <h1>How work goes here</h1>
    <p>Larger changes start as a design in records. A design is built in the order its Order section gives, one slice at a time.</p>
    <h2>Review</h2>
    <ul>
      <li>I read every change before its pull request is opened.</li>
      <li>A reply to a review thread is mine to send.</li>
    </ul>
  </>
);

export const Process: StoryObj = {
  render: () => (
    <div style={{ maxWidth: 824, margin: "0 auto", padding: 32, display: "grid", gap: 32 }}>
      <SettingsBlock>
        <SettingsProcess name="PROCESS.md" commit="4be1c2a">
          {prose}
        </SettingsProcess>
      </SettingsBlock>
      <SettingsBlock>
        <SettingsProcess name="PROCESS.md" commit="4be1c2a" uncommitted>
          {prose}
        </SettingsProcess>
      </SettingsBlock>
      <SettingsBlock>
        <SettingsProcess name="docs/how-we-work.md">{prose}</SettingsProcess>
      </SettingsBlock>
      <SettingsBlock>
        <SettingsProcessNone name="PROCESS.md" />
      </SettingsBlock>
    </div>
  ),
};
