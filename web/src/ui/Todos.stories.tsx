import type { Meta, StoryObj } from "@storybook/react-vite";
import { Todos } from "./Todos";
import { Text } from "./Text";

// An agent's todo list: partway, with the agent running (what's in hand
// turns, and reads as what it's doing) and stopped; nothing begun; all
// done.

const meta: Meta = { title: "Components/Todos" };
export default meta;

const partway = [
  { content: "Find where amounts are divided", status: "completed" },
  { content: "Give the dashboard roundDiv", status: "completed" },
  { content: "Run the tests", status: "in_progress", active: "Running the tests" },
  { content: "Write up what changed", status: "pending" },
];

export const Lists: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 20, maxWidth: 420 }}>
      <div>
        <Text as="div" dim>The agent's running</Text>
        <Todos todos={partway} live />
      </div>
      <div>
        <Text as="div" dim>Stopped partway</Text>
        <Todos todos={partway} />
      </div>
      <div>
        <Text as="div" dim>Nothing begun</Text>
        <Todos todos={partway.map((t) => ({ ...t, status: "pending" }))} />
      </div>
      <div>
        <Text as="div" dim>All done</Text>
        <Todos todos={partway.map((t) => ({ ...t, status: "completed" }))} />
      </div>
    </div>
  ),
};
