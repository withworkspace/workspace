import { describe, expect, it } from "vitest";
import type { TalkProposal } from "../api";
import { trackChanges } from "../editor/track";
import { applied, docLines, draftOf, edits, editsBySection, planAsk, planSteps, reading, sectionOf, shown } from "./briefpage";

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

const plan = `## Z
- Merged, with its tests.

## Stops early
- None.

## Steps
1. Work: **Write the proposal.** What's archived and when. "Anything that changes what customers see is proposed first"
   - in: \`records\`
   - makes: a proposal, \`proposals/archive-old-invoices.md\`
   - check: it says when an invoice is archived
   - check: \`go test ./...\` passes
2. Review: **Accept the proposal.** Read it, and say whether it's built.
3. Wait: **Tell the studios.** Before it's merged.
   - on: you
4. Wait: **The new key.** The platform team adds it.
   - on: the platform team
5. Open Pull Request
   - in: \`api\`
6. A step of no kind ws knows
`;

describe("planSteps", () => {
  const steps = planSteps(plan);

  it("lists a plan's steps as its markdown has them", () => {
    expect(steps.map((s) => [s.n, s.type, s.title])).toEqual([
      [1, "Work", "Write the proposal"],
      [2, "Review", "Accept the proposal"],
      [3, "Wait", "Tell the studios"],
      [4, "Wait", "The new key"],
      [5, "Open Pull Request", "Open Pull Request"],
      [6, undefined, "A step of no kind ws knows"],
    ]);
    expect(steps.every((s) => s.state === "todo")).toBe(true);
  });

  it("reads what's under a step, and leaves the process's words out of what it's for", () => {
    expect(steps[0]).toMatchObject({ in: "records", makes: "a proposal, `proposals/archive-old-invoices.md`", text: "Write the proposal. What's archived and when." });
    expect(steps[0].checks?.map((c) => c.text)).toEqual(["it says when an invoice is archived", "`go test ./...` passes"]);
    expect(steps[4].in).toBe("api");
  });

  it("says whose each is: yours at a checkpoint and a Wait on you, ws's at a pull request", () => {
    expect(steps.map((s) => s.who)).toEqual(["crew", "you", "you", "outside", "ws", undefined]);
  });

  it("keeps where the mission stands on a step ws has, and its checks while its name is the same", () => {
    const known = [
      { n: 1, title: "Write the proposal", text: "", state: "accepted" as const, checks: [{ text: "it says when an invoice is archived", result: "met" as const }] },
      { n: 2, title: "Read", text: "", state: "current" as const, crew: ["c2"] },
    ];
    const now = planSteps(plan, known);
    expect(now[0]).toMatchObject({ state: "accepted", checks: [{ result: "met" }] });
    expect(now[1]).toMatchObject({ state: "current", crew: ["c2"], title: "Accept the proposal" });
    expect(now[2].state).toBe("todo");
  });

  it("is nothing for a plan with no steps yet", () => {
    expect(planSteps("## Z\n- Done.\n")).toEqual([]);
    expect(planSteps("")).toEqual([]);
  });
});

describe("sectionOf", () => {
  it("is the heading a place is under, or its own", () => {
    expect(sectionOf(brief, brief.indexOf("fails one run"))).toBe("Context");
    expect(sectionOf(brief, brief.indexOf("## Acceptance"))).toBe("Acceptance");
    expect(sectionOf(brief, brief.indexOf("50 runs"))).toBe("Acceptance");
    expect(sectionOf("No headings here.\n", 3)).toBeUndefined();
    expect(sectionOf(brief, -1)).toBeUndefined();
  });
});

describe("editsBySection", () => {
  it("counts the edits still to settle under each heading", () => {
    const now = brief.replace("every time", "always").replace("- It fails one run in ten.", "- It fails one run in five.").replace("50 runs", "100 runs");
    const by = editsBySection(brief, edits(brief, now));
    expect([...by].map(([section, list]) => [section, list.length])).toEqual([
      ["Goal", 1],
      ["Context", 1],
      ["Acceptance", 1],
    ]);
    // Nothing left to settle is nothing to count.
    expect(editsBySection(brief, []).size).toBe(0);
  });
});

describe("docLines", () => {
  const at = (s: number) => new Date(Date.UTC(2026, 10, 20, 9, 0, s)).toISOString();
  const p = (n: number, over: Partial<TalkProposal>): TalkProposal => ({ n, kind: "brief", by: "agent", at: at(n * 10), state: "open", ...over });
  const turns = [
    { n: 1, by: "you", at: at(0), body: "Archive old invoices" },
    { n: 2, by: "agent", at: at(11), body: "Proposed." },
    { n: 3, by: "you", at: at(15), body: "In tax years." },
    { n: 4, by: "agent", at: at(31), body: "Proposed again, with its plan." },
  ];
  const proposals = [
    p(1, { brief: "one", state: "superseded", result: "2" }),
    p(2, { brief: "two", state: "superseded", result: "3" }),
    p(3, { brief: "two", plan: "## Z" }),
  ];

  it("counts drafts by what they say", () => {
    expect(proposals.map((x) => draftOf(proposals, x))).toEqual([1, 2, 2]);
  });

  it("lists each under the reply it came with, but for one replaced in that answer", () => {
    const { under, after } = docLines({ turns, proposals }, 3);
    expect(under.get(2)?.map((l) => [l.name, l.fate, l.shown])).toEqual([["Draft 1 of the brief", "replaced", false]]);
    expect(under.get(4)?.map((l) => [l.name, l.fate, l.shown])).toEqual([["Draft 2 of the brief, with its plan", "in the documents, to confirm", true]]);
    expect(after).toEqual([]);
  });

  it("names a revision of the plan, and lists one made since the last reply after them all", () => {
    const more = [...proposals.map((x) => ({ ...x, state: "accepted" as const })), p(9, { kind: "plan", plan: "## Z\n- More.", was: "## Z" })];
    const { after } = docLines({ turns, proposals: more }, 9);
    expect(after.map((l) => [l.name, l.shown])).toEqual([["A revision of the plan", true]]);
  });
});

describe("planAsk", () => {
  it("asks for the plan to be rewritten, or written when there's none", () => {
    expect(planAsk({ planned: true })).toBe("Rewrite the plan to fit the brief as it reads now.");
    expect(planAsk({ planned: false })).toBe("Write the plan to fit the brief as it reads now.");
  });

  it("sends a brief the planner hasn't read as it is", () => {
    const ask = planAsk({ planned: true, unseen: "## Goal\nIn tax years.\n" });
    expect(ask).toMatch(/^Rewrite the plan to fit the brief as I've changed it, and propose the two together again\./);
    expect(ask.endsWith("\n\n## Goal\nIn tax years.")).toBe(true);
  });
});
