import { describe, expect, it } from "vitest";
import type { MissionDetail } from "../api";
import { timelineOf } from "./timeline";

const t0 = Date.parse("2026-11-23T10:00:00Z");
const at = (min: number) => new Date(t0 + min * 60_000).toISOString();
const event = (min: number, kind: string, text = "", by = "") => ({ time: at(min), kind, text, by });
const crew = (id: string, min: number, ended?: number) => ({ id, started: at(min), ended: ended !== undefined, endedAt: ended === undefined ? undefined : at(ended) });

// A mission of an hour and forty minutes, looked at as it ends.
const mission = (over: Record<string, unknown>) => ({ created: at(0), state: "active", events: [], crew: [], ...over }) as unknown as MissionDetail;
const now = t0 + 100 * 60_000;

describe("timelineOf", () => {
  it("draws crew by when they started and ended, where the log says nothing of them", () => {
    const { bars } = timelineOf(mission({ crew: [crew("c1", 10, 60)] }), now);
    expect(bars).toHaveLength(1);
    expect(bars[0].from).toBeCloseTo(10);
    expect(bars[0].width).toBeCloseTo(50);
    expect(bars[0].live).toBe(false);
  });

  it("draws crew resumed later as two runs, from the log", () => {
    const events = [event(10, "crew_started", "", "c1"), event(20, "crew_ended", "", "c1"), event(70, "crew_resumed", "", "c1"), event(80, "crew_ended", "", "c1")];
    const { bars } = timelineOf(mission({ events, crew: [crew("c1", 10, 80)] }), now);
    expect(bars.map((b) => [Math.round(b.from), Math.round(b.width)])).toEqual([
      [10, 10],
      [70, 10],
    ]);
  });

  it("runs crew still at it up to now, live", () => {
    const { bars } = timelineOf(mission({ events: [event(50, "crew_started", "", "c2")], crew: [crew("c2", 50)] }), now);
    expect(bars[0].from + bars[0].width).toBeCloseTo(100);
    expect(bars[0].live).toBe(true);
    expect(bars[0].title).toMatch(/running now$/);
  });

  it("puts crew running at once in lanes, and one after another in the same", () => {
    const { bars, lanes } = timelineOf(mission({ crew: [crew("c1", 0, 40), crew("c2", 20, 60), crew("c3", 50, 70)] }), now);
    expect(bars.map((b) => b.lane)).toEqual([0, 1, 0]);
    expect(lanes).toBe(2);
  });

  it("marks what mattered, and nothing else", () => {
    const events = [event(5, "stage", "stage 1 accepted"), event(15, "stage", "stage 2 started"), event(30, "review", "approved"), event(45, "decision", "d1"), event(60, "crew_asked", "?"), event(75, "note", "by the way")];
    const { ticks } = timelineOf(mission({ events }), now);
    expect(ticks.map((t) => t.moments.map((m) => m.kind))).toEqual([["done"], ["you"], ["decided"], ["ask"]]);
    expect(ticks[0].moments[0].what).toBe("Stage accepted");
  });

  it("says step, of a mission with a plan", () => {
    const { ticks } = timelineOf(mission({ plan: {}, events: [event(5, "stage", "step 1 accepted")] }), now);
    expect(ticks[0].moments[0].what).toBe("Step accepted");
  });

  it("makes one tick of moments too close to point at apart", () => {
    const events = [event(30, "review", "approved"), event(30.2, "decision", "d1"), event(60, "crew_asked", "?")];
    const { ticks } = timelineOf(mission({ events }), now);
    expect(ticks.map((t) => t.moments.length)).toEqual([2, 1]);
  });

  it("ends a landed mission's strip at its last event, not now", () => {
    const { ticks } = timelineOf(mission({ state: "landed", events: [event(10, "decision", "d1"), event(50, "review", "approved")] }), now);
    expect(ticks.map((t) => Math.round(t.at))).toEqual([20, 100]);
  });
});
