import type { Meta, StoryObj } from "@storybook/react-vite";
import { Status } from "../ui/status";
import { MissionCrewBadge } from "./MissionCrewBadge";
import { MissionCrewCard } from "./MissionCrewCard";

// A mission's crew: their badges, each in its own colour (at rest, at
// work now, and large, as at the head of a crew's page), and what a hover
// card says of one: at work, and ended with what it last said.

const meta: Meta = { title: "Mission/MissionCrew" };
export default meta;

const working = { icon: "spinner", tone: "progress", words: "Working" } as const;
const done = { icon: "check", tone: "done", words: "Done" } as const;

export const Badges: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 14 }}>
      <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
        {["c1", "c2", "c3", "c4", "c5", "c6", "c7", "c8", "c9"].map((id) => (
          <MissionCrewBadge key={id} id={id} />
        ))}
      </div>
      <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
        <MissionCrewBadge id="c1" live />
        <MissionCrewBadge id="c2" live />
        <MissionCrewBadge id="c3" large />
        <MissionCrewBadge id="c4" large live />
      </div>
    </div>
  ),
};

const card = { display: "grid", gap: 4, width: 280, padding: "10px 12px", border: "1px solid var(--line)", borderRadius: 8, fontSize: "var(--text-sm)" } as const;

export const Cards: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "flex", gap: 20, alignItems: "start", flexWrap: "wrap" }}>
      <div style={card}>
        <MissionCrewCard
          badge={<MissionCrewBadge id="c2" live />}
          status={<Status s={working} />}
          what="claude on opus"
          capsule="api/rate-limit"
          started="10:04"
          to="/missions/rate-limit-the-public-api/crew/c2"
          more="Open its terminal"
        />
      </div>
      <div style={card}>
        <MissionCrewCard
          badge={<MissionCrewBadge id="c1" />}
          status={<Status s={done} />}
          what="claude on sonnet"
          capsule="api/rate-limit"
          started="09:12"
          ended="09:58"
          said="Each key has so many requests a minute; over it, the gateway answers 429 with when to try again."
          to="/missions/rate-limit-the-public-api/crew/c1"
          more="Read its transcript"
        />
      </div>
      <div style={card}>
        <MissionCrewCard badge={<MissionCrewBadge id="b1" />} status={<Status s={done} />} what="Briefing agent" started="09:01" ended="09:05" />
      </div>
    </div>
  ),
};
