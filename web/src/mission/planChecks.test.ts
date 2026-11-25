import { describe, expect, it } from "vitest";
import type { Stage } from "../api";
import { checksOf } from "./planChecks";

describe("checksOf", () => {
  it("puts what's not met first, with the first line of why", () => {
    const s = {
      n: 1,
      checks: [
        { text: "tests pass", result: "met" },
        { text: "the docs say how", result: "unmet", evidence: "README has nothing\nmore", kept: "run:test#2" },
        { text: "looks right", result: "yours" },
      ],
    } as unknown as Stage;
    expect(checksOf(s)).toEqual([
      { text: "the docs say how", result: "unmet", why: "README has nothing", kept: "run:test#2" },
      { text: "looks right", result: "yours", why: undefined, kept: undefined },
      { text: "tests pass", result: "met", why: undefined, kept: undefined },
    ]);
  });
  it("has none where a step has none", () => expect(checksOf({ n: 1 } as unknown as Stage)).toEqual([]));
});
