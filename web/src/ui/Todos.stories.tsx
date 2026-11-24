import type { Meta, StoryObj } from "@storybook/react-vite";
import { Todos } from "./Todos";

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
        <div className="dim">The agent's running</div>
        <Todos todos={partway} live />
      </div>
      <div>
        <div className="dim">Stopped partway</div>
        <Todos todos={partway} />
      </div>
      <div>
        <div className="dim">Nothing begun</div>
        <Todos todos={partway.map((t) => ({ ...t, status: "pending" }))} />
      </div>
      <div>
        <div className="dim">All done</div>
        <Todos todos={partway.map((t) => ({ ...t, status: "completed" }))} />
      </div>
    </div>
  ),
};
