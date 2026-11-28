import { describe, expect, it } from "vitest";
import type { Lesson } from "../api";
import { fixOf, lessonGroups, lessonOfAnchor, readingWords, seenWords } from "./lessonWords";

const now = Date.parse("2026-11-28T12:00:00Z");
const lesson = (id: number, more: Partial<Lesson>): Lesson => ({
  id,
  title: `Lesson ${id}`,
  fix: "setup",
  change: "",
  why: "",
  state: "open",
  stateAt: "2026-11-20T10:00:00Z",
  created: "2026-11-20T10:00:00Z",
  seen: [],
  crew: 1,
  missions: 1,
  lastSeen: "2026-11-27T10:00:00Z",
  ...more,
});

describe("lessonGroups", () => {
  it("puts recommendations first, then notes, quiet ones and settled ones, leaving out empty groups", () => {
    const groups = lessonGroups(
      [
        lesson(1, { crew: 3, missions: 2 }),
        lesson(2, {}),
        lesson(3, { crew: 2, lastSeen: "2026-10-01T10:00:00Z" }),
        lesson(4, { crew: 2, state: "fixed" }),
        lesson(5, { state: "dismissed" }),
      ],
      now,
    );
    expect(groups.map((g) => [g.id, g.lessons.map((l) => l.id)])).toEqual([
      ["recommended", [1]],
      ["notes", [2]],
      ["quiet", [3]],
      ["settled", [4, 5]],
    ]);
    expect(lessonGroups([lesson(2, {})], now).map((g) => g.id)).toEqual(["notes"]);
  });
});

describe("words", () => {
  it("says who ran into it", () => {
    expect(seenWords({ crew: 1, missions: 1 })).toBe("1 crew");
    expect(seenWords({ crew: 2, missions: 1 })).toBe("2 crew in 1 mission");
    expect(seenWords({ crew: 3, missions: 2 })).toBe("3 crew in 2 missions");
  });
  it("says what's to change, and a fix it doesn't know as instructions", () => {
    expect(fixOf("setup")).toBe("Capsule setup");
    expect(fixOf("you")).toBe("Yours to do");
    expect(fixOf("nonsense")).toBe("Agent instructions");
  });
  it("says how reading stands", () => {
    expect(readingWords({ on: true, read: 31, failed: 0, waiting: 4 })).toBe("31 handoffs read · 4 waiting");
    expect(readingWords({ on: true, read: 1, failed: 1, waiting: 0 })).toBe("1 handoff read · 1 unreadable");
    expect(readingWords({ on: false, read: 0, failed: 0, waiting: 3 })).toBe("Reading handoffs is off");
  });
  it("reads a lesson's anchor", () => {
    expect(lessonOfAnchor("#lesson-3")).toBe(3);
    expect(lessonOfAnchor("#other")).toBeUndefined();
  });
});
