import { describe, expect, it } from "vitest";
import type { Notification } from "../api";
import { byDay, sourceIcon, unreadOf } from "./feed";

const n = (num: number, at: string, over: Partial<Notification> = {}): Notification => ({
  n: num,
  ref: `n${num}`,
  at,
  source: "missions",
  kind: "mission.ended",
  text: "Mission cache ended",
  read: false,
  ...over,
});

describe("the feed", () => {
  it("is drawn by day, the latest first, each day in the feed's order", () => {
    const days = byDay([n(4, "2026-11-25T15:00:00"), n(3, "2026-11-25T09:00:00"), n(2, "2026-11-24T18:00:00"), n(1, "2026-11-20T08:00:00")]);
    expect(days.map((d) => d.notifications.map((x) => x.n))).toEqual([[4, 3], [2], [1]]);
    expect(new Set(days.map((d) => d.key)).size).toBe(3);
  });

  it("has no days when it's empty", () => {
    expect(byDay([])).toEqual([]);
  });

  it("says which are unread", () => {
    expect(unreadOf([n(3, "2026-11-25T09:00:00"), n(2, "2026-11-25T08:00:00", { read: true }), n(1, "2026-11-24T08:00:00")])).toEqual([3, 1]);
  });

  it("draws a notification by where it's from, and one that went wrong as an alert", () => {
    expect(sourceIcon({ source: "pulls" })).toBe("pr");
    expect(sourceIcon({ source: "plugins" })).toBe("box");
    expect(sourceIcon({ source: "ws" })).toBe("info");
    expect(sourceIcon({ source: "missions", attention: true })).toBe("alert");
    expect(sourceIcon({ source: "something-new" })).toBe("bell");
  });
});
