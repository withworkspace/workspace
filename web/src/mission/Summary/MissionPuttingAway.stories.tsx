import type { Meta, StoryObj } from "@storybook/react-vite";
import { MissionPuttingAway } from "./MissionPuttingAway";

// What archiving left to do: two capsules burned in a moment, then one
// that couldn't be, with why.

const meta: Meta = { title: "Mission/MissionPuttingAway" };
export default meta;

const none = () => {};

export const Waiting: StoryObj = {
  render: () => (
    <div className="page narrow">
      <MissionPuttingAway
        items={[
          { id: 1, doing: "burning api/cache-work", state: "waiting" },
          { id: 2, doing: "putting away the capsules t2 lifted", state: "waiting" },
        ]}
        onRetry={none}
        onLeave={none}
      />
    </div>
  ),
};

export const Failed: StoryObj = {
  render: () => (
    <div className="page narrow">
      <MissionPuttingAway
        items={[
          { id: 3, doing: "burning web/invoice-totals", state: "failed", error: "'repos/web/invoice-totals' contains modified or untracked files, use --force to delete it" },
          { id: 4, doing: "burning api/cache-work", state: "sending" },
        ]}
        onRetry={none}
        onLeave={none}
      />
    </div>
  ),
};
