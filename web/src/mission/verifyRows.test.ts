import { describe, expect, it } from "vitest";
import type { Bundle } from "../api";
import { allClearWords, bundleGroup, checkRows, nothingLeftWords, queued, rowKey, rowLine, sendable, shown, shownBundle, standing, stillLeft, tally, toApprove, waiting } from "./verifyRows";

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
  it("else the first that's yours to approve: what a step waits on, then the newest", () => {
    expect(shownBundle(bundles)?.id).toBe("b3");
    expect(shownBundle([...bundles, bundle({ id: "b4", replacedBy: "b5" })])?.id).toBe("b3");
    expect(shownBundle([bundle({ id: "b1", asked: true, askedAt: 2 }), ...bundles.slice(1)])?.id).toBe("b1");
  });
  it("is none for a mission with none, or with none left to approve", () => {
    expect(shownBundle([])).toBeUndefined();
    expect(shownBundle([bundle({ id: "b1", verdict: "approved" }), bundle({ id: "b2", replacedBy: "b3" }), bundle({ id: "b3", verdict: "sent back" })])).toBeUndefined();
  });
});

describe("the queue of a mission's bundles", () => {
  const bundles = [
    bundle({ id: "b1", verdict: "approved" }),
    bundle({ id: "b2", verdict: "approved", stale: true }),
    bundle({ id: "b3", replacedBy: "b5" }),
    bundle({ id: "b4", verdict: "sent back" }),
    bundle({ id: "b5", asked: true, askedAt: 6 }),
    bundle({ id: "b6", stale: true }),
    bundle({ id: "b7" }),
    bundle({ id: "b8", stale: true, asked: true, askedAt: 6 }),
  ];
  it("says where each stands", () => {
    expect(bundles.map(bundleGroup)).toEqual(["approved", "stale", "stale", "back", "todo", "stale", "todo", "todo"]);
  });
  it("is to approve, sent back, approved, then stale: newest first, what a step waits on before the rest", () => {
    expect(queued(bundles).map((g) => [g.group, g.bundles.map((b) => b.id)])).toEqual([
      ["todo", ["b8", "b5", "b7"]],
      ["back", ["b4"]],
      ["approved", ["b1"]],
      ["stale", ["b6", "b3", "b2"]],
    ]);
    expect(toApprove(bundles).map((b) => b.id)).toEqual(["b8", "b5", "b7"]);
  });
  it("says where they stand when none is left to approve", () => {
    expect(nothingLeftWords(bundles.filter((b) => bundleGroup(b) !== "todo"))).toBe(
      "Bundle approved. 1 sent back to crew: the next one will wait for you. 3 replaced or stale.",
    );
    expect(nothingLeftWords([bundle({ id: "b1", verdict: "approved" }), bundle({ id: "b2", verdict: "approved" })])).toBe("All 2 bundles approved.");
  });
  it("has no group that's empty", () => {
    expect(queued([bundle({ id: "b1", verdict: "approved" })]).map((g) => g.group)).toEqual(["approved"]);
    expect(toApprove([bundle({ id: "b1", verdict: "approved" })])).toEqual([]);
  });
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
    expect(rows.map((r) => rowLine(browser, r))).toEqual(["Can't tell, c1", "Met, c1: the relevant lines. The judge says unmet", "Met, c1: 2 images, 1 recording", "Met, run by Workspace"]);
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
    expect(standing(bundle({ verdict: "approved", stale: true }))).toBe("Approved, now stale");
    expect(standing(bundle({ verdict: "sent back" }))).toBe("Sent back");
    expect(standing(bundle({ verdict: "sent back", replacedBy: "b2" }))).toBe("Replaced by b2");
    expect(standing(bundle({ stale: true }))).toBe("Stale");
    expect(standing(browser)).toBe("3 met, 1 can't tell");
  });
  it("marks the checks you've looked at", () => {
    const rows = checkRows({ ...browser, looked: ["c3", "r2"] });
    expect(rows.filter((r) => r.looked).map((r) => r.key)).toEqual(["c3", "r2"]);
  });
  it("puts a check left for you that you've approved after those you haven't, and counts it as no longer left", () => {
    const rows = checkRows({ ...browser, looked: ["c3"] });
    expect(rows.map((r) => r.key)).toEqual(["c4", "c3", "c1", "r2"]);
    expect(stillLeft(rows).map((r) => r.key)).toEqual(["c4"]);
    expect(stillLeft(checkRows({ ...browser, looked: ["c3", "c4"] }))).toEqual([]);
  });
  it("says you're clear once the last is approved, and what stands as it came out", () => {
    const b = { ...browser, looked: ["c3", "c4"] };
    expect(allClearWords(b, checkRows(b))).toBe("You've approved both checks left for you in b3. 1 can't tell stands as it is: approving passes b3 with it.");
    const judged = bundle({ id: "b9", checks: [{ n: 1, check: "it reloads", verdict: "met", said: "it did", judged: { check: "it reloads", result: "unmet", by: "the judge" } }] });
    expect(allClearWords(judged, checkRows(judged))).toBe("You've approved the one check left for you in b9.");
  });
  it("sends back the notes that say something, by their checks", () => {
    expect(sendable({ 3: " it's gone in Firefox ", 1: "", 0: "try it narrow" })).toEqual([
      { check: 0, text: "try it narrow" },
      { check: 3, text: "it's gone in Firefox" },
    ]);
    expect(sendable(undefined)).toEqual([]);
  });
});
