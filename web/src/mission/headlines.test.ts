import { describe, expect, it } from "vitest";
import type { Headline } from "../api";
import { shownHeadlines, splitHeadline } from "./headlines";

const h = (court: Headline["court"], kind: string): Headline => ({
  court,
  kind,
  group: court === "agent" ? "in-progress" : court === "outside" ? "waiting" : "needs-you",
  label: kind,
  text: kind,
  action: { label: "" },
});

const kinds = (hs: Headline[]) => hs.map((x) => x.kind);

describe("the headlines a mission's page shows", () => {
  it("shows one of the agents', the first, however many it's given", () => {
    const given = [h("agent", "working"), h("agent", "launching"), h("agent", "briefing"), h("agent", "deciding")];
    expect(kinds(shownHeadlines(given))).toEqual(["working"]);
  });
  it("lists every one of yours and of outside's, in the order they came", () => {
    const given = [
      h("you", "crew-asks"),
      h("you", "flag"),
      h("agent", "launching"),
      h("agent", "briefing"),
      h("outside", "checks-running"),
      h("outside", "waiting-on"),
    ];
    expect(kinds(shownHeadlines(given))).toEqual(["crew-asks", "flag", "launching", "checks-running", "waiting-on"]);
  });
  it("changes nothing of what ws gives: one of the agents' already", () => {
    const given = [h("agent", "working"), h("outside", "awaiting-review")];
    expect(shownHeadlines(given)).toEqual(given);
    expect(shownHeadlines([h("you", "ready")])).toHaveLength(1);
    expect(shownHeadlines([])).toEqual([]);
  });
});

describe("a headline's title", () => {
  const said = (text: string) => splitHeadline({ ...h("you", "step-review"), text }, "Review");

  it("is what its sentence says before its colon, and the rest follows", () => {
    expect(said("Looks finished: everything in the brief is met")).toEqual(["Looks finished", "Everything in the brief is met"]);
  });
  it("isn't cut at a colon inside brackets", () => {
    const text =
      "Step 3 is done (c1: Fixed: tax is rounded once for each rate). Step 4 is yours, to review: Read the fix. Before its pull request, which ws opens once your review approves it.";
    expect(said(text)).toEqual(["Review", text]);
  });
  it("is in its first sentence, or it has none", () => {
    const text = "Step 3 is done. Step 4 is yours, to review: Read the fix, before its pull request, which ws opens once your review approves it.";
    expect(said(text)).toEqual(["Review", text]);
    expect(said("v1.4.0 is out: tagged this morning")).toEqual(["v1.4.0 is out", "Tagged this morning"]);
  });
  it("is the whole of a short sentence whose only colon is inside brackets", () => {
    expect(said("Step 1 is done (c1: reproduced). Go on.")).toEqual(["Step 1 is done (c1: reproduced). Go on.", undefined]);
  });
  it("is the headline's words for a long sentence with no colon", () => {
    const text = "a".repeat(120);
    expect(said(text)).toEqual(["Review", text]);
  });
});
