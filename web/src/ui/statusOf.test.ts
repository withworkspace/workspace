import { describe, expect, it } from "vitest";
import { crewStatus } from "./statusOf";

const words = (c: Parameters<typeof crewStatus>[0]) => crewStatus(c).words;

describe("where crew stand, in a list of crew", () => {
  it("says starting while ws's mark stands, and only then", () => {
    expect(words({ ended: false, starting: true })).toBe("Starting");
    // Resumed: what they last did is from the session before.
    expect(words({ ended: false, starting: true, activity: "waiting" })).toBe("Starting");
    expect(words({ ended: true, starting: true })).toBe("Ended");
  });
  it("takes crew who've reported nothing, and aren't starting, as working: as the mission's headline does", () => {
    expect(words({ ended: false })).toBe("Working");
  });
  it("says what crew reported once they've started", () => {
    expect(words({ ended: false, activity: "working" })).toBe("Working");
    expect(words({ ended: false, activity: "waiting" })).toBe("Waiting for you");
    expect(words({ ended: false, activity: "waiting", waitingOn: "the tests" })).toBe("Waiting on its work");
    expect(words({ ended: false, activity: "permission" })).toBe("Needs permission");
    expect(words({ ended: false, status: "asking" })).toBe("Asks you");
    expect(words({ ended: false, status: "done" })).toBe("Reported done");
  });
});
