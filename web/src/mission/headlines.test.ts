import { describe, expect, it } from "vitest";
import type { Headline } from "../api";
import { shownHeadlines } from "./headlines";

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
