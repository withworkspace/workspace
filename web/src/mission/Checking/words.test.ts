import { describe, expect, it } from "vitest";
import { checkingTitle, checkNumbers, clock, limit } from "./words";

describe("what's running while ws checks a step", () => {
  it("says how long, to the second", () => {
    expect(clock(0)).toBe("0s");
    expect(clock(45_400)).toBe("45s");
    expect(clock(80_000)).toBe("1m 20s");
    expect(clock(3_725_000)).toBe("1h 2m");
    expect(clock(-5)).toBe("0s");
  });
  it("says a step's limit as the plan does", () => {
    expect(limit(15 * 60_000)).toBe("15m");
    expect(limit(60 * 60_000)).toBe("1h");
    expect(limit(90 * 60_000)).toBe("1h30m");
  });
  it("names checks by number", () => {
    expect(checkNumbers([2])).toBe("check 2");
    expect(checkNumbers([4, 5])).toBe("checks 4 and 5");
    expect(checkNumbers([2, 4, 5])).toBe("checks 2, 4 and 5");
  });
  it("titles a command, or the judge", () => {
    const at = { step: 3, of: 5, since: "2026-11-17T10:00:00Z", until: "2026-11-17T10:15:00Z" };
    expect(checkingTitle({ ...at, n: 2, command: "just test" })).toBe("Check 2 of 5 for step 3");
    expect(checkingTitle({ ...at, judged: [4, 5], judge: "the judge (claude:sonnet)" })).toBe("The judge (claude:sonnet) is reading checks 4 and 5 of step 3");
  });
});
