import { describe, expect, it } from "vitest";
import { type Tracked, trackChanges } from "./track";

// What a tracked change shows: what goes and what comes, by whose change,
// and where each change ends.
const shown = (t: Tracked) => ({
  spans: t.spans.map((s) => `${s.kind}:${t.doc.textBetween(s.from, s.to)}@${s.id}`),
  wholes: t.wholes.map((w) => `${w.kind}:${t.doc.nodeAt(w.pos)!.textContent}@${w.id}`),
});

describe("tracked changes", () => {
  const doc = "# Plan\n\nShip on Friday, after the review.\n\n- one\n- two\n\n## Later\n\nMaybe.\n";

  it("shows words changed in a paragraph, what goes beside what comes", () => {
    const t = trackChanges(doc, [{ id: "t1", from: "Ship on Friday,", to: "Ship on Tuesday," }]);
    expect(shown(t)).toEqual({ spans: ["del:Friday,@t1", "ins:Tuesday,@t1"], wholes: [] });
    expect(t.doc.child(1).textContent).toBe("Ship on Friday,Tuesday, after the review.");
    // Its Accept and Decline go at the end of the paragraph.
    const para = t.doc.child(0).nodeSize;
    expect(t.ends).toEqual([{ id: "t1", pos: para + t.doc.child(1).nodeSize - 1, inline: true }]);
    expect(t.placed).toEqual(["t1"]);
  });

  it("shows a phrase reworded as one change, not word by word", () => {
    const t = trackChanges("One file, no tests of its own. The end.\n", [{ id: "t1", from: "no tests of its own.", to: "and one test that checks it." }]);
    expect(shown(t).spans).toEqual(["del:no tests of its own.@t1", "ins:and one test that checks it.@t1"]);
  });

  it("shows blocks put in and taken out whole", () => {
    const added = trackChanges(doc, [{ id: "t2", from: "Maybe.", to: "Maybe.\n\nOr never." }]);
    expect(shown(added)).toEqual({ spans: [], wholes: ["ins:Or never.@t2"] });
    const gone = trackChanges(doc, [{ id: "t3", from: "## Later\n\nMaybe.\n", to: "" }]);
    expect(shown(gone)).toEqual({ spans: [], wholes: ["del:Later@t3", "del:Maybe.@t3"] });
    expect(gone.ends[0].inline).toBe(false);
  });

  it("follows a list down to the item that changed", () => {
    const t = trackChanges(doc, [{ id: "t4", from: "- two", to: "- three" }]);
    expect(shown(t)).toEqual({ spans: ["del:two@t4", "ins:three@t4"], wholes: [] });
    const items = trackChanges(doc, [{ id: "t5", from: "- two\n", to: "- two\n- three\n" }]);
    expect(shown(items)).toEqual({ spans: [], wholes: ["ins:three@t5"] });
  });

  it("keeps each change's own, and leaves out what can't be placed", () => {
    const t = trackChanges(doc, [
      { id: "t6", from: "Maybe.", to: "Perhaps." },
      { id: "t7", from: "# Plan", to: "# The Plan" },
      { id: "t8", from: "not in it", to: "x" },
      { id: "t9", from: "- one\n- two", to: "- uno" },
      { id: "t10", from: "one", to: "eins" },
    ]);
    const of = (id: string) => t.spans.filter((s) => s.id === id).map((s) => `${s.kind}:${t.doc.textBetween(s.from, s.to)}`);
    expect(of("t9")).toEqual(["del:one", "ins:uno"]);
    expect(t.wholes.filter((w) => w.id === "t9").map((w) => `${w.kind}:${t.doc.nodeAt(w.pos)!.textContent}`)).toEqual(["del:two"]);
    expect(t.spans.filter((s) => s.id === "t7").map((s) => `${s.kind}:${t.doc.textBetween(s.from, s.to)}`)).toEqual(["ins:The "]);
    expect(t.spans.filter((s) => s.id === "t6").map((s) => `${s.kind}:${t.doc.textBetween(s.from, s.to)}`)).toEqual(["del:Maybe.", "ins:Perhaps."]);
    expect(t.unplaced.sort()).toEqual(["t10", "t8"]);
    expect(t.placed.sort()).toEqual(["t6", "t7", "t9"]);
  });
});
