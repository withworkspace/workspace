import type { Meta, StoryObj } from "@storybook/react-vite";
import { SelectionToolbar } from "./SelectionToolbar";

// What's offered over a passage picked out of a document: Ask about this
// (a), Explain, and Link where there's a section to link to; and the same
// with no Link; and Ask alone, as a mission's brief offers it (its planner
// is who's asked). It stands over the middle of the passage, in the window.

const meta: Meta = { title: "Ask/SelectionToolbar" };
export default meta;

const none = () => {};

export const OverAPassage: StoryObj = {
  render: () => (
    <div style={{ height: 280, padding: 24, background: "var(--bg)" }}>
      <SelectionToolbar left={260} top={90} onAsk={none} onExplain={none} onLink={none} />
      <SelectionToolbar left={260} top={170} onAsk={none} onExplain={none} />
      <SelectionToolbar left={260} top={250} onAsk={none} askTitle="Ask the planner about this passage" />
    </div>
  ),
};
