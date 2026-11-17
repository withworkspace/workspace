import { describe, expect, it } from "vitest";
import { matchQuote } from "./anchor";

// Where a quote is, as the reader and the editor both find it: of more than
// one place, the one whose surroundings match best, whatever its spaces.
describe("matchQuote", () => {
  const text = "Cache per user. Then cache per user again, keyed by tenant.";
  it("finds the only place", () => {
    expect(matchQuote(text, { exact: "keyed  by\ntenant" })).toEqual([43, 58]);
  });
  it("picks the place whose surroundings match", () => {
    expect(matchQuote(text, { exact: "per user", prefix: "Then cache " })).toEqual([27, 35]);
    expect(matchQuote(text, { exact: "per user", suffix: ". Then" })).toEqual([6, 14]);
  });
  it("takes a bonus over the words around it", () => {
    expect(matchQuote(text, { exact: "per user", prefix: "Then cache " }, (i) => (i === 6 ? 1000 : 0))).toEqual([6, 14]);
  });
  it("finds nothing that isn't there", () => {
    expect(matchQuote(text, { exact: "per team" })).toBeUndefined();
    expect(matchQuote(text, { exact: "  " })).toBeUndefined();
  });
});
