import type { Meta, StoryObj } from "@storybook/react-vite";
import { toast } from "../ui/toast";
import { MissionDrop } from "./MissionDrop";

// Giving a mission up, from beside its state: the quiet button, and what
// it opens, a line for why (if you want to say) and the button that drops
// it.

const meta: Meta = { title: "Mission/MissionDrop" };
export default meta;

const said = (why: string) => toast(why ? `Dropped: ${why}` : "Dropped");

export const Shut: StoryObj = {
  render: () => (
    <div className="page">
      <MissionDrop onDrop={said} />
    </div>
  ),
};

export const Open: StoryObj = {
  render: () => (
    <div className="page" style={{ minHeight: 120, display: "flex", justifyContent: "end", alignItems: "start" }}>
      <MissionDrop open onDrop={said} />
    </div>
  ),
};

export const Dropping: StoryObj = {
  render: () => (
    <div className="page" style={{ minHeight: 120, display: "flex", justifyContent: "end", alignItems: "start" }}>
      <MissionDrop open busy onDrop={said} />
    </div>
  ),
};
