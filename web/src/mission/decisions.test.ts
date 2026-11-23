import { describe, expect, it } from "vitest";
import type { Decision, Stage } from "../api";
import { decisionsPath, decisionsWords, groupDecisions, stepOf } from "./decisions";

const d = (id: string, by: string, more: Partial<Decision> = {}): Decision => ({ id, by, what: `What ${id}`, at: `2026-11-0${id.slice(1)}T10:00:00Z`, state: "current", ...more }) as Decision;
const stage = (n: number, crew: string[] = []): Stage => ({ n, title: `Step ${n}`, text: "", state: "todo", crew }) as Stage;

describe("a mission's decisions, as its page lays them out", () => {
  const stages = [stage(1, ["c1"]), stage(2, ["c2"]), stage(3, ["c1"])];

  it("has yours apart, the latest first", () => {
    const g = groupDecisions([d("d1", "you"), d("d2", "c1"), d("d3", "you")], stages);
    expect(g.yours.map((x) => x.id)).toEqual(["d3", "d1"]);
    expect(g.worked).toBe(1);
  });

  it("has the rest by step in the plan's order, as they were made, those at none last", () => {
    const g = groupDecisions([d("d5", "orchestrator"), d("d4", "c2", { step: 2 }), d("d3", "c1", { step: 1 }), d("d2", "c2", { step: 2 }), d("d1", "c1", { step: 3 })], stages);
    expect(g.work.map((s) => [s.n, s.step?.title, s.list.map((x) => x.id)])).toEqual([
      [1, "Step 1", ["d3"]],
      [2, "Step 2", ["d2", "d4"]],
      [3, "Step 3", ["d1"]],
      [0, undefined, ["d5"]],
    ]);
  });

  it("keeps what no longer stands out of both, the latest first", () => {
    const g = groupDecisions([d("d1", "you", { state: "superseded", supersededBy: "d3" }), d("d2", "c1", { state: "withdrawn" }), d("d3", "you")], stages);
    expect(g.yours.map((x) => x.id)).toEqual(["d3"]);
    expect(g.work).toEqual([]);
    expect(g.gone.map((x) => x.id)).toEqual(["d2", "d1"]);
  });

  it("takes the step ws kept, else the latest its crew was on", () => {
    expect(stepOf(d("d1", "c1", { step: 1 }), stages)).toBe(1);
    expect(stepOf(d("d1", "c1"), stages)).toBe(3);
    expect(stepOf(d("d1", "you"), stages)).toBe(0);
    // A step the plan no longer has is still the step it was made at.
    expect(groupDecisions([d("d1", "c9", { step: 7 })], stages).work).toMatchObject([{ n: 7, step: undefined }]);
  });

  it("says how many stand, and whose", () => {
    expect(decisionsWords({ yours: [d("d1", "you")], worked: 11 })).toBe("1 of yours and 11 from the work");
    expect(decisionsWords({ yours: [], worked: 2 })).toBe("2 from the work");
    expect(decisionsWords({ yours: [], worked: 0 })).toBe("");
  });

  it("names a decision's place on its mission's page", () => {
    expect(decisionsPath("cut the tag", "d4")).toBe("/missions/cut%20the%20tag?tab=decisions&d=d4");
    expect(decisionsPath("m")).toBe("/missions/m?tab=decisions");
  });
});
