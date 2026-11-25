import type { Meta, StoryObj } from "@storybook/react-vite";
import { Chip } from "./Chip";
import { LiveDot } from "./LiveDot";

// A live dot: in a chip that says who's working, and alone, in the colour
// of what it's in.

const meta: Meta = { title: "Components/LiveDot" };
export default meta;

export const Dots: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "flex", gap: 16, alignItems: "center" }}>
      <Chip tone="progress">
        <LiveDot />
        c2 working
      </Chip>
      <span style={{ display: "flex", color: "var(--green)" }}>
        <LiveDot />
      </span>
      <span style={{ display: "flex", color: "var(--dim)" }}>
        <LiveDot />
      </span>
    </div>
  ),
};
