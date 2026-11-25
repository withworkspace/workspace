import { describe, expect, it } from "vitest";
import type { Notification } from "../api";
import { tellingOf, toTell } from "./telling";

const n = (num: number, more: Partial<Notification> = {}): Notification =>
  ({ n: num, ref: `n${num}`, at: "2026-11-25T12:00:00Z", source: "plugin:jira", kind: "jira:assigned", text: `Notification ${num}`, read: false, desktop: true, to: `/x/jira/issue/RAM-${num}`, ...more }) as Notification;

describe("toTell", () => {
  const feed = [n(5), n(4, { desktop: false }), n(3, { read: true }), n(2, { to: "/missions/cache" }), n(1)];

  it("is what's new, unread and asked to be told of", () => {
    expect(toTell(feed, 1, true, "/").map((x) => x.n)).toEqual([5, 2]);
  });

  it("with the page in front, leaves out what's about the page you're on", () => {
    expect(toTell(feed, 1, false, "/missions/cache").map((x) => x.n)).toEqual([5]);
    // Hidden, you're not looking at it: it's told.
    expect(toTell(feed, 1, true, "/missions/cache").map((x) => x.n)).toEqual([5, 2]);
  });
});

describe("tellingOf", () => {
  it("is nothing for nothing", () => {
    expect(tellingOf([])).toBeUndefined();
  });

  it("says the one, opening its page", () => {
    expect(tellingOf([n(7)])).toEqual({ text: "Notification 7", label: "Open", n: 7, to: "/x/jira/issue/RAM-7" });
  });

  it("shows the bell's list for one with no page, and for several", () => {
    expect(tellingOf([n(7, { to: undefined })])).toEqual({ text: "Notification 7", label: "Show" });
    expect(tellingOf([n(7), n(6), n(5)])).toEqual({ text: "3 new notifications", label: "Show" });
  });
});
