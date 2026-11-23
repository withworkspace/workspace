import type { Meta, StoryObj } from "@storybook/react-vite";
import { Chip } from "../ui/Chip";
import { Status } from "../ui/status";
import { MissionCapsuleCard } from "./MissionCapsuleCard";
import { MissionDecisionCard } from "./MissionDecisionCard";
import { MissionDocCard } from "./MissionDocCard";
import { MissionRefChip } from "./MissionRefChip";
import { MissionTalkCard } from "./MissionTalkCard";

// The names a mission's page knows, in words that mention them (a chip
// for a short one, underlined words for a longer one), and what a hover
// card says of each: a decision (and one that's been replaced), a
// conversation, a capsule, a document and an artifact.

const meta: Meta = { title: "Mission/MissionRefs" };
export default meta;

export const Chips: StoryObj = {
  render: () => (
    <div className="page">
      It was decided in <MissionRefChip>d2</MissionRefChip>, after <MissionRefChip>t3</MissionRefChip>: the limit is in{" "}
      <MissionRefChip words mono>
        api/rate-limit
      </MissionRefChip>
      , and{" "}
      <MissionRefChip words>docs/limits.md</MissionRefChip> says so.
    </div>
  ),
};

const card = { display: "grid", gap: 4, width: 280, padding: "10px 12px", border: "1px solid var(--line)", borderRadius: 8, fontSize: "var(--text-sm)" } as const;
const yours = { icon: "reply", tone: "attention", words: "Your turn" } as const;

export const Cards: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "flex", gap: 20, alignItems: "start", flexWrap: "wrap" }}>
      <div style={card}>
        <MissionDecisionCard id="d2" what="Limits are per key, not per customer." why="A customer with ten keys has ten scripts; one of them shouldn't starve the rest." by="You" at="2h ago" />
      </div>
      <div style={card}>
        <MissionDecisionCard id="d1" stands="Replaced" what="Limits are per customer." by="c1" at="yesterday" />
      </div>
      <div style={card}>
        <MissionTalkCard
          title="Should a burst over the limit be queued or refused?"
          status={<Status s={yours} />}
          summary="Refused, with when to try again: a queue hides the limit from whoever hit it."
          to="/missions/rate-limit-the-public-api/conversations/t3"
        />
      </div>
      <div style={card}>
        <MissionCapsuleCard
          name="api/rate-limit"
          branch="ws/rate-limit"
          badges={<Chip tone="done">checks success</Chip>}
          review="/missions/rate-limit-the-public-api/review/api/rate-limit"
          files="/missions/rate-limit-the-public-api/files/api/rate-limit"
        />
      </div>
      <div style={card}>
        <MissionDocCard title="How limits work" kind="handoff" state={<Chip tone="quiet">private</Chip>} desc="Each key has so many requests a minute; over it, the gateway answers 429." to="/missions/rate-limit-the-public-api/artifact?ref=x" />
      </div>
      <div style={card}>
        <MissionDocCard artifact title="Limits, as a table" kind="artifact" versions={3} state={<Chip tone="done">promoted</Chip>} to="/artifacts/a4" />
      </div>
    </div>
  ),
};
