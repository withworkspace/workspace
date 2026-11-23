import type { Meta, StoryObj } from "@storybook/react-vite";
import type { Call } from "./Calls";
import { Doing } from "./Doing";

// A run going now, as one line: before its first step (thinking, or what
// the page says it's at), at a step, and at one of many, whose count
// opens on them; and in a narrow place, where a long step is cut short.

const meta: Meta = { title: "Components/Doing" };
export default meta;

const root = "/Users/sam/sorrel";
const steps: Call[] = [
  { tool: "Read", input: `${root}/api/invoice.go`, result: "package api" },
  { tool: "Grep", input: "roundDiv", result: "api/invoice.go:3" },
  { tool: "Bash", input: `cd ${root} && go test ./api/... -run 'TestShare|TestRound' -count=1 -v` },
];

export const Lines: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 18, maxWidth: 520 }}>
      <Doing calls={[]} />
      <Doing calls={[]} idle="Reading the page" />
      <Doing calls={steps.slice(0, 1)} root={root} />
      <Doing calls={steps} root={root} />
      <div style={{ maxWidth: 240 }}>
        <div className="dim">In a narrow place</div>
        <Doing calls={steps} root={root} />
      </div>
    </div>
  ),
};
