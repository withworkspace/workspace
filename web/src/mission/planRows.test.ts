import { describe, expect, it } from "vitest";
import type { Bundle, MissionEvent, Stage } from "../api";
import { checksWords, foldNote, planWindow, stepBundle, stepText, stepTimes, waitHow, watchWords, worthOpening } from "./planRows";

// A plan of n steps at step `at` (0: none started; n + 1: all done).
function plan(n: number, at: number, over: Record<number, Partial<Stage>> = {}): Stage[] {
  return Array.from({ length: n }, (_, i) => {
    const k = i + 1;
    return { n: k, title: `Step ${k}`, text: `Step ${k}. What it does.`, state: k < at ? "accepted" : k === at ? "current" : "todo", ...over[k] } as Stage;
  });
}
const ns = (list: Stage[]) => list.map((s) => s.n);

describe("planWindow", () => {
  it("lists a short plan whole", () => {
    const w = planWindow(plan(9, 5));
    expect(ns(w.shown)).toEqual([1, 2, 3, 4, 5, 6, 7, 8, 9]);
    expect(w.earlier).toEqual([]);
    expect(w.later).toEqual([]);
  });

  it("shows two before the step it's on and three after, and folds the rest", () => {
    const w = planWindow(plan(27, 9));
    expect(ns(w.earlier)).toEqual([1, 2, 3, 4, 5, 6]);
    expect(ns(w.shown)).toEqual([7, 8, 9, 10, 11, 12]);
    expect(w.later).toHaveLength(15);
    expect(w.later[0].n).toBe(13);
  });

  it("doesn't fold a run of one", () => {
    // At step 4, only step 1 would be folded before: it's shown.
    expect(ns(planWindow(plan(27, 4)).shown)).toEqual([1, 2, 3, 4, 5, 6, 7]);
    // At step 23, only step 27 would be folded after.
    expect(ns(planWindow(plan(27, 23)).shown)).toEqual([21, 22, 23, 24, 25, 26, 27]);
  });

  it("starts at the top before anything's begun, and ends at the bottom once it's done", () => {
    const fresh = planWindow(plan(27, 0));
    expect(ns(fresh.shown)).toEqual([1, 2, 3, 4]);
    expect(fresh.earlier).toEqual([]);
    const done = planWindow(plan(27, 28));
    expect(ns(done.shown)).toEqual([25, 26, 27]);
    expect(done.later).toEqual([]);
  });

  it("is round the first step to come when none is current", () => {
    const stages = plan(27, 28).map((s) => (s.n >= 14 ? { ...s, state: "todo" as const } : s));
    expect(ns(planWindow(stages).shown)).toEqual([12, 13, 14, 15, 16, 17]);
  });
});

describe("foldNote", () => {
  it("says what's before is all done, or how many were passed with a check unmet", () => {
    expect(foldNote(plan(6, 7), "earlier")).toEqual({ text: "all done" });
    const passed = plan(6, 7, { 4: { round: 1, checks: [{ text: "tests pass", result: "can't tell" }, { text: "it builds", result: "met" }] } });
    expect(foldNote(passed, "earlier")).toEqual({ text: "1 passed unmet", tone: "warn" });
  });

  it("says how many to come are yours, and which is next", () => {
    const later = plan(27, 9, { 16: { who: "you" }, 19: { who: "you" }, 23: { who: "you" } }).slice(12);
    expect(foldNote(later, "later")).toEqual({ text: "3 yours, next 16", tone: "yours" });
    expect(foldNote(plan(27, 9).slice(12), "later")).toBeUndefined();
  });
});

describe("worthOpening", () => {
  const step = (over: Partial<Stage>) => ({ n: 1, title: "A step", text: "A step.", state: "current", ...over }) as Stage;

  it("opens a step that makes something, has crew, checks or a clock", () => {
    expect(worthOpening(step({ type: "Work", who: "crew", makes: "the amended Plan" }))).toBe(true);
    expect(worthOpening(step({ type: "Verify", who: "crew", checks: [{ text: "it reads well" }] }))).toBe(true);
    expect(worthOpening(step({ type: "Work", who: "crew", crew: ["c2"] }))).toBe(true);
    expect(worthOpening(step({ type: "Work", who: "crew", limit: "1h" }))).toBe(true);
  });

  it("leaves a step that's only its kind and whose it is as a row", () => {
    expect(worthOpening(step({ type: "Open Pull Request", who: "ws", whoSays: "ws", opensPR: true, crew: [] }))).toBe(false);
    expect(worthOpening(step({ type: "Review", who: "you", whoSays: "you" }))).toBe(false);
  });
});

describe("stepTimes", () => {
  const event = (time: string, text: string, kind = "stage"): MissionEvent => ({ time, kind, text });

  it("reads when each step began and was accepted from the log", () => {
    const times = stepTimes([
      event("2026-11-17T19:18:00Z", "on step 1: **Reproduce gaps.** Confirm…"),
      event("2026-11-17T19:36:00Z", "step 1 accepted: **Reproduce gaps.** Confirm…"),
      event("2026-11-17T19:36:01Z", "on step 2: **Mark handoffs.**"),
      event("2026-11-17T19:40:00Z", "step 12 was mentioned in a note", "note"),
    ]);
    expect(times.get(1)).toEqual({ from: "2026-11-17T19:18:00Z", to: "2026-11-17T19:36:00Z" });
    expect(times.get(2)).toEqual({ from: "2026-11-17T19:36:01Z" });
    expect(times.has(12)).toBe(false);
  });

  it("times a step started again from its last start, and reads a brief's stages", () => {
    const times = stepTimes([event("t1", "on stage 3: build it"), event("t2", "on stage 3: build it"), event("t3", "stage 3 accepted: build it")]);
    expect(times.get(3)).toEqual({ from: "t2", to: "t3" });
  });
});

describe("stepBundle", () => {
  const bundle = (id: string, step: number, over: Partial<Bundle> = {}) => ({ id, step, crew: "c1", at: "t", summary: "", checks: [], ...over }) as Bundle;

  it("is the latest bundle of the step that nothing has replaced", () => {
    const bundles = [bundle("b1", 1), bundle("b3", 4, { replacedBy: "b5" }), bundle("b5", 4, { verdict: "approved" }), bundle("b6", 5)];
    expect(stepBundle(bundles, 4)?.id).toBe("b5");
    expect(stepBundle(bundles, 2)).toBeUndefined();
  });
});

describe("stepText", () => {
  it("drops the title a step's text begins with", () => {
    expect(stepText({ n: 4, title: "Build ui-lib", text: "Build ui-lib. Build the package from the design.", state: "todo" })).toBe("Build the package from the design.");
    expect(stepText({ n: 1, title: "Design", text: "Write the design as a Plan.", state: "todo" })).toBe("Write the design as a Plan.");
  });
});

describe("waitHow", () => {
  const wait = (over: Partial<Stage>): Stage => ({ n: 2, title: "Release", text: "Release. It's released.", state: "current", type: "Wait", ...over });

  it("says a Wait with a check is watched, and passes by itself", () => {
    expect(waitHow(wait({ on: "npm", passes: "watched" }))).toMatchObject({ auto: true, word: "Watched" });
    expect(waitHow(wait({ on: "you", passes: "watched" }))?.says).toContain("or mark it done");
  });

  it("says a Wait without one is manual, and who moves it", () => {
    expect(waitHow(wait({ on: "you", passes: "done" }))).toEqual({ auto: false, word: "Manual", says: "nothing's watching for it: it moves when you mark it done" });
    expect(waitHow(wait({ on: "the platform team", passes: "said" }))?.says).toBe("nothing's watching for it: it moves when someone marks it done");
    // One from before a Wait said who it's on.
    expect(waitHow(wait({}))).toMatchObject({ auto: false, word: "Manual" });
  });

  it("is nothing for any other kind of step", () => {
    expect(waitHow(wait({ type: "Work" }))).toBeUndefined();
  });

  it("opens the Wait the plan is on: how it's passed is the thing to know", () => {
    expect(worthOpening(wait({ on: "you", passes: "done" }))).toBe(true);
  });

  it("says what ws's last look came to", () => {
    const at = () => "14:02";
    expect(watchWords(wait({ passes: "watched" }), at)).toBe("ws hasn't looked yet");
    expect(watchWords(wait({ passes: "watched", watch: { since: "x", looked: "y", looks: 3, came: "not yet" } }), at)).toBe("ws last looked 14:02: not yet");
  });
});

describe("checksWords", () => {
  const step = (over: Partial<Stage>): Stage => ({ n: 3, title: "Verify", text: "Verify. It works.", state: "current", type: "Verify", ...over });
  const check = (result?: "met" | "unmet") => ({ text: "It works", result }) as NonNullable<Stage["checks"]>[number];
  const at = () => "14:02";

  it("says how many checks there are before they've run", () => {
    expect(checksWords(step({ checks: [check()] }), at)).toBe("1 check, not run yet");
    expect(checksWords(step({ checks: [check(), check()] }), at)).toBe("2 checks, not run yet");
  });

  it("says how many were met in the latest round", () => {
    expect(checksWords(step({ round: 2, checks: [check("met"), check("unmet")] }), at)).toBe("Checks, round 2: 1 of 2 met");
  });

  it("says a Wait's as ws last looked, and once it's passed, as ws last saw them", () => {
    const wait = step({ type: "Wait", passes: "watched", checks: [check()] });
    expect(checksWords(wait, at)).toBe("ws hasn't looked yet");
    expect(checksWords({ ...wait, state: "accepted" }, at)).toBe("Its check, as ws last saw it");
    expect(checksWords({ ...wait, state: "accepted", checks: [check(), check()] }, at)).toBe("Its checks, as ws last saw them");
  });
});
