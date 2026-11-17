import { describe, expect, it } from "vitest";
import type { TalkProposal } from "../api";
import { trackChanges } from "../editor/track";
import { applied, edits, reading, shown } from "./briefpage";

const brief = `# Flaky test

## Goal
The flaky test passes every time.

## Context
- It fails one run in ten.
- Only on CI.

## Acceptance
- It passes 50 runs in a row.
- None.
`;

// Each case is the brief, and the brief as the agent's next draft has it.
const cases: [string, string][] = [
  ["a word", brief.replace("every time", "every single time")],
  ["a line added in a list", brief.replace("- Only on CI.\n", "- Only on CI.\n- Since #120.\n")],
  ["a line taken out", brief.replace("- Only on CI.\n", "")],
  ["a section added at the end", brief + "\n## Stops early\n- It can't be reproduced.\n"],
  ["a line added at the start", "Raised from w24.\n\n" + brief],
  ["a line like another changed", brief.replace("- None.", "- No new sleeps.")],
  ["several places", brief.replace("every time", "always").replace("- Only on CI.", "- On CI and locally.").replace("50 runs", "100 runs")],
  ["all of it", "## Goal\nSomething else entirely.\n"],
  ["nothing", brief],
];

describe("edits", () => {
  it.each(cases)("turns the draft before into the next: %s", (_, now) => {
    const list = edits(brief, now);
    expect(applied(brief, list)).toBe(now);
    if (now === brief) expect(list).toEqual([]);
  });

  it.each(cases)("are each found once, to show as tracked changes: %s", (_, now) => {
    const list = edits(brief, now);
    const t = trackChanges(
      brief,
      list.map((e) => ({ ...e })),
    );
    expect(t.unplaced).toEqual([]);
  });

  it("leave what's declined as it was", () => {
    const now = brief.replace("every time", "always").replace("50 runs", "100 runs");
    const list = edits(brief, now);
    expect(list).toHaveLength(2);
    const r = reading(brief, list, { [list[0].id]: "decline", [list[1].id]: "accept" });
    expect(r.pending).toEqual([]);
    expect(r.result).toBe(brief.replace("50 runs", "100 runs"));
  });

  it("are read part-way: what's accepted is in, what's left still to settle", () => {
    const now = brief.replace("every time", "always").replace("50 runs", "100 runs");
    const list = edits(brief, now);
    const r = reading(brief, list, { [list[0].id]: "accept" });
    expect(r.result).toBeUndefined();
    expect(r.pending.map((e) => e.id)).toEqual([list[1].id]);
    expect(r.base).toContain("always");
    expect(r.comes).toBe(now); // what's left to read is taken as proposed
    expect(trackChanges(r.base, r.pending).unplaced).toEqual([]);
  });
});

const proposal = (n: number, p: Partial<TalkProposal>): TalkProposal => ({ n, kind: "brief", by: "agent", at: `2026-11-16T18:0${n}:00Z`, state: "open", ...p });

describe("shown", () => {
  const plan = "## Z\n- Fixed.\n## Steps\n1. Work: fix it.\n";
  it("is the brief and plan as they are, with nothing proposed", () => {
    const s = shown({ state: "active", brief, plan }, { proposals: [] });
    expect(s).toMatchObject({ phase: "underway", brief: { text: brief }, plan: { text: plan }, draft: 0 });
    expect(s.proposal).toBeUndefined();
  });

  it("briefing, is the first draft with nothing to compare it with", () => {
    const s = shown({ state: "briefing", brief: "# Flaky test\n", briefUnwritten: true }, { proposals: [proposal(1, { brief, plan, was: "# Flaky test\n" })] });
    expect(s).toMatchObject({ phase: "briefing", draft: 1, brief: { text: brief }, plan: { text: plan } });
    expect(s.brief.was).toBeUndefined();
    expect(s.plan.was).toBeUndefined();
  });

  const turn = (n: number, by: string, at: string) => ({ n, by, at, body: "" });
  it("briefing, compares the latest draft with the one you could have read before it, which it replaces", () => {
    const next = brief.replace("every time", "always");
    const turns = [turn(1, "you", "2026-11-16T18:00:00Z"), turn(2, "agent", "2026-11-16T18:01:30Z"), turn(3, "you", "2026-11-16T18:02:30Z")];
    const s = shown({ state: "briefing", brief: "", briefUnwritten: true }, { turns, proposals: [proposal(1, { brief, plan }), proposal(3, { brief: next, plan: plan + "2. Review: check it.\n" })] });
    expect(s).toMatchObject({ draft: 2, brief: { text: next, was: brief }, plan: { was: plan } });
    expect(s.proposal?.n).toBe(3);
    expect(s.others.map((p) => p.n)).toEqual([1]);
  });

  it("briefing, a draft proposed again in the same answer, with its plan, is still the first", () => {
    const turns = [turn(1, "you", "2026-11-16T18:00:00Z"), turn(2, "agent", "2026-11-16T18:03:00Z")];
    const s = shown({ state: "briefing", brief: "", briefUnwritten: true }, { turns, proposals: [proposal(1, { brief }), proposal(2, { brief, plan })] });
    expect(s).toMatchObject({ draft: 1, brief: { text: brief }, plan: { text: plan } });
    expect(s.brief.was).toBeUndefined();
    expect(s.plan.was).toBeUndefined();
  });

  it("briefing, a plan proposed alone comes with the brief as it's written", () => {
    const s = shown({ state: "briefing", brief, plan: "" }, { proposals: [proposal(1, { kind: "plan", plan, was: "" })] });
    expect(s.brief).toEqual({ text: brief });
    expect(s.plan).toEqual({ text: plan });
  });

  it("under way, is an edit to the brief as it is, stale once the brief's changed", () => {
    const next = brief.replace("every time", "always");
    expect(shown({ state: "active", brief, plan }, { proposals: [proposal(1, { brief: next, was: brief })] }).brief).toEqual({ text: next, was: brief, stale: false });
    expect(shown({ state: "active", brief: brief + "x", plan }, { proposals: [proposal(1, { brief: next, was: brief })] }).brief.stale).toBe(true);
    // One proposed against an empty brief (a draft left from briefing it).
    expect(shown({ state: "active", brief, plan }, { proposals: [proposal(1, { brief: next, plan })] })).toMatchObject({ brief: { stale: true }, plan: { stale: true } });
  });

  it("under way, a plan revision is compared with the plan it revises", () => {
    const next = plan + "2. Review: check it.\n";
    const s = shown({ state: "active", brief, plan }, { proposals: [proposal(1, { kind: "plan", plan: next, was: plan })] });
    expect(s.brief).toEqual({ text: brief });
    expect(s.plan).toEqual({ text: next, was: plan, stale: false });
  });

  it("leaves settled proposals be", () => {
    const s = shown({ state: "active", brief, plan }, { proposals: [proposal(1, { brief: "x", state: "accepted" }), proposal(2, { kind: "decision", what: "y" })] });
    expect(s.proposal).toBeUndefined();
    expect(s.draft).toBe(1);
  });
});
