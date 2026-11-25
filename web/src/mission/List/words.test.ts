import { describe, expect, it } from "vitest";
import type { HeadlineGroup, Mission } from "../../api";
import { onRoute, sectionsOf } from "./words";

const mission = (slug: string, group: HeadlineGroup) => ({ slug, headline: { group } }) as Mission;

describe("the missions list", () => {
  it("groups missions by whose court they're in, what needs you first", () => {
    const ms = [mission("a", "up-next"), mission("b", "needs-you"), mission("c", "past"), mission("d", "needs-you"), mission("e", "waiting")];
    expect(sectionsOf(ms, false).map((s) => [s.title, s.missions.map((m) => m.slug)])).toEqual([
      ["Needs you", ["b", "d"]],
      ["Waiting", ["e"]],
      ["Up next", ["a"]],
    ]);
  });

  it("shows the past last, when asked", () => {
    const ms = [mission("c", "past"), mission("b", "needs-you")];
    expect(sectionsOf(ms, true).at(-1)).toEqual({ title: "Past", missions: [ms[0]], past: true });
    expect(sectionsOf([mission("c", "past")], false)).toEqual([]);
  });

  it("says where a mission is on its route in a few words", () => {
    expect(onRoute("Step 2 of 7: Verify, crew at work")).toBe("Step 2 of 7");
    expect(onRoute("Stage 1 of 3")).toBe("Stage 1 of 3");
    expect(onRoute("Every step done: at Z")).toBe("At Z");
    expect(onRoute("Being briefed")).toBe("");
  });
});
