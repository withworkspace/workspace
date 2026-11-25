import { describe, expect, it } from "vitest";
import type { MissionCard } from "../api";
import { actHref, cameFrom, docLink } from "./hrefs";

const d = { slug: "fix-it", planner: "t1" };
const card = (kind: string) => ({ kind }) as unknown as MissionCard;

describe("actHref", () => {
  it.each([
    ["crew", "c2", "/missions/fix-it/crew/c2"],
    ["review", "api/fix-it", "/missions/fix-it/review/api/fix-it"],
    ["verify", "b1", "/missions/fix-it?tab=verify&b=b1"],
    ["read", "mission:handoffs/c1.md", "/missions/fix-it/artifact?ref=mission%3Ahandoffs%2Fc1.md"],
    ["talk", "t2", "/missions/fix-it/conversations/t2"],
    ["talk", "t1", "/missions/fix-it?tab=brief"],
    ["brief-page", "", "/missions/fix-it?tab=brief"],
    ["log", "", "/missions/fix-it?tab=log"],
  ])("%s %s", (verb, target, to) => expect(actHref(d, card("asked"), { do: verb, target })).toBe(to));
  it("leads to the planner's conversation for something else it proposed", () =>
    expect(actHref(d, card("proposed"), { do: "talk", target: "t1" })).toBe("/missions/fix-it/conversations/t1"));
  it("has nowhere for an act done here", () => expect(actHref(d, card("asked"), { do: "accept" })).toBeUndefined());
});

describe("where else", () => {
  it("reads a document", () => expect(docLink("fix-it", "mission:brief.md")).toBe("/missions/fix-it/artifact?ref=mission%3Abrief.md"));
  it("goes back to the conversation it came from", () => {
    expect(cameFrom("w3")).toBe("/conversations/w3");
    expect(cameFrom("cache/t4")).toBe("/missions/cache/conversations/t4");
  });
});
