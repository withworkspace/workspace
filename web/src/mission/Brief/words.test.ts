import { describe, expect, it } from "vitest";
import { acceptLabel, proposedName, proposedTitle, quoted, suggestions, unplaced, yourWords } from "./words";

describe("quoted and yourWords", () => {
  it("quote a passage over what's said of it, and take it apart again", () => {
    const body = quoted("It passes 50 runs\nin a row.", "Why 50?");
    expect(body).toBe("> It passes 50 runs\n> in a row.\n\nWhy 50?");
    expect(yourWords(body)).toEqual({ passage: "It passes 50 runs in a row.", said: "Why 50?" });
  });

  it("leave a turn that quotes nothing as it was said", () => {
    expect(yourWords("Is the plan still right?")).toEqual({ said: "Is the plan still right?" });
    expect(yourWords("> not a quote: nothing's said of it")).toEqual({ said: "> not a quote: nothing's said of it" });
  });
});

describe("suggestions", () => {
  it("are about the passage in hand, whoever's there to ask", () => {
    expect(suggestions({ held: true, planner: false, briefing: true })[0]).toBe("Why does it say this?");
  });

  it("are none with no planner and no passage", () => {
    expect(suggestions({ held: false, planner: false, briefing: false })).toEqual([]);
  });

  it("are of the brief while it's briefed, and of the plan once it's under way", () => {
    expect(suggestions({ held: false, planner: true, briefing: true })).toContain("What's still open?");
    expect(suggestions({ held: false, planner: true, briefing: false })).toContain("Is the plan still right?");
  });
});

describe("what a proposal's called", () => {
  it("is a draft or the plan while it's briefed", () => {
    expect(proposedName({ briefing: true, kind: "brief", draft: 2 })).toBe("Draft 2");
    expect(proposedName({ briefing: true, kind: "brief", draft: 0 })).toBe("Draft 1");
    expect(proposedName({ briefing: true, kind: "plan", draft: 2 })).toBe("The plan");
    expect(proposedTitle({ briefing: true, kind: "brief", draft: 2, plan: true })).toBe("Draft 2 of the brief, with its plan");
    expect(proposedTitle({ briefing: true, kind: "brief", draft: 1 })).toBe("Draft 1 of the brief");
    expect(proposedTitle({ briefing: true, kind: "plan", draft: 1 })).toBe("The plan, proposed");
  });

  it("is an edit or a revision once it's under way", () => {
    expect(proposedName({ briefing: false, kind: "brief", draft: 1 })).toBe("Brief edit");
    expect(proposedName({ briefing: false, kind: "plan", draft: 1 })).toBe("Plan revision");
    expect(proposedTitle({ briefing: false, kind: "brief", draft: 1 })).toBe("An edit to the brief");
    expect(proposedTitle({ briefing: false, kind: "plan", draft: 1 })).toBe("A revision of the plan");
  });
});

describe("acceptLabel", () => {
  it("confirms and starts a mission being briefed, unless it's a plan for a brief not written", () => {
    expect(acceptLabel({ briefing: true, kind: "brief" })).toBe("Confirm and start");
    expect(acceptLabel({ briefing: true, kind: "plan" })).toBe("Confirm and start");
    expect(acceptLabel({ briefing: true, kind: "plan", unwritten: true })).toBe("Accept the plan");
  });

  it("accepts what's proposed once it's under way", () => {
    expect(acceptLabel({ briefing: false, kind: "plan" })).toBe("Accept the plan");
    expect(acceptLabel({ briefing: false, kind: "brief" })).toBe("Accept the edit");
  });
});

describe("unplaced", () => {
  it("says one change or how many", () => {
    expect(unplaced(1)).toBe("A change couldn't be shown in place: it's in as proposed.");
    expect(unplaced(3)).toBe("3 changes couldn't be shown in place: they're in as proposed.");
  });
});
