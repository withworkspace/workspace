import { describe, expect, it } from "vitest";
import type { Bundle } from "../api";
import { checkRows, listed, rowKey, rowLine, sendable, shown, shownBundle, standing, tally, waiting } from "./verifyRows";

const bundle = (over: Partial<Bundle>): Bundle => ({ id: "b1", crew: "c1", at: "2026-11-17T10:00:00Z", summary: "x", checks: [], ...over });

const browser = bundle({
  id: "b3",
  replaces: "b2",
  checks: [
    {
      n: 1,
      check: "a pick applies at once",
      verdict: "met",
      said: "it did",
      shows: [
        { name: "a.png", url: "/a", kind: "image" },
        { name: "b.png", url: "/b", kind: "image" },
        { name: "pick.webm", url: "/p", kind: "video" },
      ],
    },
    { n: 3, check: "a pick reaches other tabs", verdict: "can't tell", said: "one tab only" },
    { n: 4, check: "it survives a reload", verdict: "met", said: "it did", lines: ["web/a.ts:1-9"], judged: { check: "it survives a reload", result: "unmet", by: "the judge" } },
  ],
  ran: [{ n: 2, check: "`npm test` passes", result: "met", by: "ws" }],
});

describe("the bundle the tab shows", () => {
  const bundles = [bundle({ id: "b1" }), bundle({ id: "b2", replacedBy: "b3" }), browser];
  it("is the one picked", () => expect(shownBundle(bundles, "b2")?.id).toBe("b2"));
  it("else the latest nothing's taken the place of", () => {
    expect(shownBundle(bundles)?.id).toBe("b3");
    expect(shownBundle([...bundles, bundle({ id: "b4", replacedBy: "b5" })])?.id).toBe("b3");
  });
  it("is none for a mission with none", () => expect(shownBundle([])).toBeUndefined());
  it("lists those standing first, newest first", () => expect(listed(bundles).map((b) => b.id)).toEqual(["b3", "b1", "b2"]));
});

describe("a bundle's checks", () => {
  const rows = checkRows(browser);
  it("puts what's left for you first, then what's settled, in the step's order", () => {
    expect(rows.map((r) => [r.key, r.left])).toEqual([
      ["c3", true],
      ["c4", true], // met, but the judge didn't find it so
      ["c1", false],
      ["r2", false],
    ]);
  });
  it("names each among the page's items", () => expect(rowKey(browser, rows[0])).toBe("v:b3:c3"));
  it("says each in a line", () => {
    expect(rows.map((r) => rowLine(browser, r))).toEqual(["Can't tell, c1", "Met, c1: the lines it's done in. The judge says unmet", "Met, c1: 2 images, 1 recording", "Met, ws ran it"]);
  });
  it("is titled by its own claim where the step has no checks", () => {
    expect(checkRows(bundle({ checks: [{ n: 1, verdict: "unmet", said: "dark keeps the old accent" }] }))[0].title).toBe("dark keeps the old accent");
  });
  it("counts what they came to, the commands ws ran among them", () =>
    expect(tally(browser)).toEqual([
      ["met", 3],
      ["can't tell", 1],
    ]));
  it("says what shows one", () => {
    expect(shown({ n: 1, verdict: "met", said: "x", shows: [{ name: "o.txt", url: "/o", kind: "text" }] })).toBe("1 transcript");
    expect(shown({ n: 1, verdict: "unmet", said: "x" })).toBe("");
  });
});

describe("where a bundle stands with you", () => {
  it("waits for you while a Review step looks at it and you've said nothing", () => {
    expect(waiting(bundle({ asked: true, askedAt: 2 }))).toBe(true);
    expect(waiting(bundle({}))).toBe(false);
    expect(waiting(bundle({ asked: true, verdict: "sent back" }))).toBe(false);
    expect(waiting(bundle({ asked: true, replacedBy: "b2" }))).toBe(false);
  });
  it("is said in a few words", () => {
    expect(standing(bundle({ asked: true }))).toBe("Waiting for you");
    expect(standing(bundle({ asked: true, verdict: "approved" }))).toBe("Approved by you");
    expect(standing(bundle({ verdict: "sent back" }))).toBe("Sent back");
    expect(standing(bundle({ verdict: "sent back", replacedBy: "b2" }))).toBe("Replaced by b2");
    expect(standing(bundle({ stale: true }))).toBe("Stale");
    expect(standing(browser)).toBe("3 met, 1 can't tell");
  });
  it("marks the checks you've looked at", () => {
    const rows = checkRows({ ...browser, looked: ["c3", "r2"] });
    expect(rows.filter((r) => r.looked).map((r) => r.key)).toEqual(["c3", "r2"]);
  });
  it("sends back the notes that say something, by their checks", () => {
    expect(sendable({ 3: " it's gone in Firefox ", 1: "", 0: "try it narrow" })).toEqual([
      { check: 0, text: "try it narrow" },
      { check: 3, text: "it's gone in Firefox" },
    ]);
    expect(sendable(undefined)).toEqual([]);
  });
});
