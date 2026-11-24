import { describe, expect, it } from "vitest";
import type { MissionDetail } from "../api";
import merging from "./fixtures/sorrel/missions/rate-limit-the-public-api.json";
import { peekOf } from "./peek";

const d = merging as unknown as MissionDetail;

describe("a mission's peek", () => {
  it("says what it's for from its brief's goal, or what was asked before there's one", () => {
    expect(peekOf(d).goal).toBe("One customer's script can't slow the api for everyone else.");
    expect(peekOf({ ...d, brief: "", briefUnwritten: true, request: "Limit each key" }).goal).toBe("Limit each key");
    expect(peekOf({ ...d, brief: "", request: undefined }).goal).toBeUndefined();
  });

  it("has its cards as what it stands on, without what they offer", () => {
    expect(peekOf(d).stands).toEqual([{ tone: "attention", icon: "pr", title: "api#46 is ready to merge", detail: "It's approved and its checks pass", nothing: undefined }]);
  });

  it("has every step of its plan as it stands", () => {
    const steps = peekOf(d).steps;
    expect(steps.map((s) => s.n)).toEqual(d.stages.map((s) => s.n));
    expect(steps.every((s, i) => s.state === d.stages[i].state && s.title === d.stages[i].title)).toBe(true);
  });

  it("has the crew that haven't ended, each with the way to its page", () => {
    expect(peekOf(d).crew).toEqual([{ id: "c1", status: { icon: "check-circle", tone: "done", words: "Reported done" }, what: "claude", capsule: "api/rate-limit", to: "/missions/rate-limit-the-public-api/crew/c1" }]);
    expect(peekOf({ ...d, crew: d.crew.map((c) => ({ ...c, ended: true })) }).crew).toEqual([]);
  });

  it("has its capsules, each with its pull request's review here and how that stands", () => {
    expect(peekOf(d).capsules).toEqual([{ name: "api/rate-limit", gone: false, pr: { name: "api#46", to: "/reviews/api/46", status: { icon: "pr", tone: "quiet", words: "Open" } } }]);
    const failing = peekOf({ ...d, capsules: d.capsules.map((c) => ({ ...c, exists: false, pr: c.pr && { ...c.pr, checks: "failure" } })) }).capsules[0];
    expect(failing.gone).toBe(true);
    expect(failing.pr?.status.words).toBe("Checks failing");
  });
});
