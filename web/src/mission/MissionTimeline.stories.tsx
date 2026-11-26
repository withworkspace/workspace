import type { Meta, StoryObj } from "@storybook/react-vite";
import type { MissionDetail } from "../api";
import { MissionMomentCard } from "./MissionMomentCard";
import { MissionTimeline, type TimelineBar, type TimelineTick } from "./MissionTimeline";
import { timelineOf } from "./timeline";
import { Text } from "../ui/Text";

// A mission's timeline: the strip alone (a page that's loading), one crew
// at work, crew in lanes with one running now, and ticks of each kind
// (one of them several moments at once). Then what a tick's card says,
// drawn open; and the strips of missions as ws gave them (fixtures/),
// each as it stood at its last event.

const meta: Meta = { title: "Mission/MissionTimeline" };
export default meta;

const bar = (key: string, hue: number, from: number, width: number, lane = 0, live = false): TimelineBar => ({ key, hue, from, width, lane, live, title: `${key}: 10:04 to 10:31 (27m)` });

const moments = {
  done: { kind: "done", what: "Step accepted", when: "10:31", text: "step 1 accepted: **Limit each key.** Before its pull request." },
  you: { kind: "you", what: "Reviewed", when: "10:40", text: "reviewed `api/rate-limit`: approved (r1)" },
  decided: { kind: "decided", what: "Decided", when: "10:12", text: "d1: limits are per key, not per customer" },
  ask: { kind: "ask", what: "Asked", when: "10:20", text: "c1 asks: should a burst over the limit be queued or refused?" },
} as const;

const ticks: TimelineTick[] = [
  { at: 12, moments: [moments.decided] },
  { at: 34, moments: [moments.ask] },
  { at: 58, moments: [moments.done, moments.you] },
  { at: 81, moments: [moments.you] },
];

const row = { display: "grid", gap: 6 } as const;

export const Strips: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 28 }}>
      <div style={row}>
        <Text dim>Nothing yet</Text>
        <MissionTimeline />
      </div>
      <div style={row}>
        <Text dim>One crew, done</Text>
        <MissionTimeline bars={[bar("c1", 228, 4, 38)]} />
      </div>
      <div style={row}>
        <Text dim>Crew in lanes, one running now</Text>
        <MissionTimeline lanes={3} bars={[bar("c1", 228, 2, 40), bar("c2", 152, 20, 35, 1), bar("c3", 32, 48, 52, 2, true), bar("c1 again", 228, 62, 20)]} />
      </div>
      <div style={row}>
        <Text dim>With what mattered</Text>
        <MissionTimeline lanes={2} bars={[bar("c1", 228, 2, 55), bar("c2", 152, 40, 60, 1, true)]} ticks={ticks} />
      </div>
    </div>
  ),
};

export const Moments: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 16, width: 340 }}>
      <div>
        <MissionMomentCard moments={[moments.ask]} />
      </div>
      <div>
        <MissionMomentCard moments={[moments.done, moments.you, moments.decided]} />
      </div>
    </div>
  ),
};

const missions = import.meta.glob<MissionDetail>("./fixtures/*/missions/*.json", { eager: true, import: "default" });

export const OfMissions: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 20 }}>
      {Object.entries(missions).map(([path, d]) => {
        const last = d.events.at(-1)?.time ?? d.created;
        return (
          <div key={path} style={row}>
            <Text dim>{d.title}</Text>
            <MissionTimeline {...timelineOf(d, new Date(last).getTime() + 90_000)} />
          </div>
        );
      })}
    </div>
  ),
};
