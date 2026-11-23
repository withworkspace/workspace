import type { Meta, StoryObj } from "@storybook/react-vite";
import { Excerpt } from "./Excerpt";

// Excerpts: a few lines numbered from where they start, a line alone,
// and one too long for its box, which scrolls.

const meta: Meta = { title: "Components/Excerpt" };
export default meta;

export const Lines: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 6, maxWidth: 520 }}>
      <Excerpt lines={["func total(lines []Line) Money {", "\tvar sum Money", "\tfor _, l := range lines {", "\t\tsum += l.Amount.Round()", "\t}"]} from={41} />
      <Excerpt lines={["export const tallest = 220;"]} from={7} title="The lines it's on" />
      <Excerpt lines={["", `const placeholder = "Describe the mission you have in mind: what, and why. The agent reads the code and proposes a brief, which is yours to change.";`]} from={182} />
    </div>
  ),
};
