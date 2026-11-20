import type { Meta, StoryObj } from "@storybook/react-vite";
import { Button } from "./Button";
import { Chip } from "./Chip";
import { Count } from "./Count";
import { GroupHead } from "./GroupHead";
import { PageHead } from "./PageHead";
import { Summary, SummaryRow } from "./Summary";

// A page's parts: its head, with what's beside the title and its actions;
// a group's heading with its count; counts alone; a summary.

const meta: Meta = { title: "Library/Page parts" };
export default meta;

export const Heads: StoryObj = {
  render: () => (
    <div className="page">
      <PageHead title="Runs" />
      <PageHead title="Runs">
        <Chip tone="done">2 running</Chip>
      </PageHead>
      <PageHead
        title="Missions"
        actions={
          <>
            <Button variant="ghost" size="small">
              Show past
            </Button>
            <Button variant="primary">New mission</Button>
          </>
        }
      />
      <section>
        <GroupHead count={3}>Running</GroupHead>
        <p className="dim">A group's rows go here.</p>
      </section>
      <section>
        <GroupHead count="3 of 12">Found</GroupHead>
      </section>
      <section>
        <GroupHead>No count</GroupHead>
      </section>
    </div>
  ),
};

export const Counts: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "flex", gap: 8 }}>
      <Count>3</Count>
      <Count>128</Count>
      <Count tone="attention">2</Count>
    </div>
  ),
};

export const Summaries: StoryObj = {
  render: () => (
    <div className="page">
      <Summary>
        <SummaryRow label="Names">cssmod.localhost and each run below it</SummaryRow>
        <SummaryRow label="Router">On :80, held by the daemon</SummaryRow>
        <SummaryRow label="Settings">Local names, in Settings</SummaryRow>
      </Summary>
    </div>
  ),
};
