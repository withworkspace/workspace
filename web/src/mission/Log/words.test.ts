import { describe, expect, it } from "vitest";
import type { MissionEvent } from "../../api";
import { logShows } from "./words";

const ev = (e: Partial<MissionEvent>): MissionEvent => ({ time: "2026-11-16T17:28:00Z", kind: "note", text: "", ...e });

describe("logShows", () => {
  const crew = ev({ text: "c1 lifted demo/fix", by: "c1" });
  const you = ev({ text: "you approved b1" });
  const thread = ev({ kind: "thread", text: "you commented on the brief" });
  it("shows everything", () => expect([crew, you, thread].every((e) => logShows(e, "all", ""))).toBe(true));
  it("shows one who's", () => {
    expect(logShows(crew, "crew", "")).toBe(true);
    expect(logShows(you, "crew", "")).toBe(false);
    expect(logShows(you, "you", "")).toBe(true);
  });
  it("shows threads", () => {
    expect(logShows(thread, "threads", "")).toBe(true);
    expect(logShows(you, "threads", "")).toBe(false);
  });
  it("searches what it says, in any case", () => {
    expect(logShows(crew, "all", "LIFTED")).toBe(true);
    expect(logShows(crew, "all", "landed")).toBe(false);
  });
});
