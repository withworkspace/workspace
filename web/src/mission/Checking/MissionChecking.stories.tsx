import type { Meta, StoryObj } from "@storybook/react-vite";
import { checkingCommand, checkingJudge, checkingQuiet } from "../samples";
import { CheckingLine } from "./CheckingLine";
import { CheckingNow } from "./CheckingNow";

// What ws is running while it checks a step: a command that's printed, one
// that hasn't yet, and the judge reading; under a headline, and in a line
// of the missions list.

const meta: Meta = { title: "Mission/MissionChecking" };
export default meta;

const all = [checkingCommand, checkingQuiet, checkingJudge];

export const Now: StoryObj = {
  render: () => (
    <div style={{ display: "grid", gap: 24, width: 520, padding: 16, background: "var(--bg)" }}>
      {all.map((c, i) => (
        <CheckingNow key={i} c={c} />
      ))}
    </div>
  ),
};

export const Line: StoryObj = {
  render: () => (
    <div style={{ display: "grid", gap: 8, width: 720, padding: 16, background: "var(--bg)", fontSize: "var(--text-sm)" }}>
      {all.map((c, i) => (
        <div key={i}>
          <CheckingLine c={c} />
        </div>
      ))}
    </div>
  ),
};
