import { describe, expect, it } from "vitest";
import type { TalkDetail, TalkProposal } from "../api";
import { anchorChip, citedSection, inCard, linesChip, passageOf, placed, waiting } from "./ask";

const sections = [
  { anchor: "how-it-lands", text: "How it lands" },
  { anchor: "whats-open", text: "What's open" },
  { anchor: "notes", text: "Notes" },
  { anchor: "notes-1", text: "Notes" },
];

describe("citedSection", () => {
  it("is the section an anchor names", () => {
    expect(citedSection("#how-it-lands", sections)).toBe("how-it-lands");
    expect(citedSection("#notes-1", sections)).toBe("notes-1");
    expect(citedSection("#How-It-Lands", sections)).toBe("how-it-lands");
  });
  it("is the section named by its words, as GitHub would anchor them", () => {
    expect(citedSection("#What's open", sections)).toBe("whats-open");
    expect(citedSection("#what%27s%20open", sections)).toBe("whats-open");
  });
  it("is a section named nearly so", () => {
    expect(citedSection("#how-it", sections)).toBe("how-it-lands");
    expect(citedSection("#how-it-lands-2", sections)).toBe("how-it-lands");
  });
  it("is none for what isn't a section, or isn't a link to one", () => {
    expect(citedSection("#rollout", sections)).toBeUndefined();
    expect(citedSection("#", sections)).toBeUndefined();
    expect(citedSection("docs/x.md#notes", sections)).toBeUndefined();
    expect(citedSection("#no", sections)).toBeUndefined();
  });
});

describe("passageOf", () => {
  it("is the text as it reads, with a little either side", () => {
    expect(passageOf("  We cache\n tokens  ", "x".repeat(50) + " Before it,", " after it.", { anchor: "caching", title: "Caching" })).toEqual({
      exact: "We cache tokens",
      prefix: "xxxxxxxxxxxxxxxxxxxxx Before it,",
      suffix: "after it.",
      section: { anchor: "caching", title: "Caching" },
    });
  });
  it("is nothing for a stray click", () => {
    expect(passageOf(" a ", "", "")).toBeUndefined();
  });
  it("is kept to what's sent", () => {
    expect(passageOf("word ".repeat(2000), "", "")?.exact.length).toBe(4000);
  });
});

const talk = (t: Partial<TalkDetail>): TalkDetail => ({ id: "w1", kind: "talk", by: "you", at: "", status: "open", turns: [], ...t });
const turn = (by: string) => ({ n: 1, by, at: "", body: "x" });

describe("waiting", () => {
  it("is while the agent's at it, or hasn't started", () => {
    expect(waiting(talk({ turns: [turn("you")] }))).toBe(true);
    expect(waiting(talk({ answering: true, turns: [turn("you"), turn("agent")] }))).toBe(true);
  });
  it("isn't once it's answered, or said it couldn't", () => {
    expect(waiting(talk({ turns: [turn("you"), turn("agent")] }))).toBe(false);
    expect(waiting(talk({ turns: [turn("you"), turn("ws")] }))).toBe(false);
    expect(waiting(talk({ status: "resolved", turns: [turn("you")] }))).toBe(false);
    expect(waiting(undefined)).toBe(false);
  });
  it("isn't for a card settled quietly after the answer, and is for one that wakes the agent", () => {
    const card = (quiet: boolean) => ({ ...turn("you"), n: 3, origin: "proposal:1", quiet });
    expect(waiting(talk({ turns: [turn("you"), turn("agent"), card(true)] }))).toBe(false);
    expect(waiting(talk({ turns: [turn("you"), turn("agent"), card(false)] }))).toBe(true);
  });
});

describe("anchorChip", () => {
  it("is the passage an ask was of, quoted", () => {
    expect(anchorChip({ doc: "records:notes/tokens.md", quote: { exact: "We cache\n tokens per user." } })).toMatchObject({ kind: "passage", text: "“We cache tokens per user.”", title: "We cache tokens per user." });
  });
  it("is the lines of a pull request it was of, by the file's name", () => {
    expect(anchorChip({ pr: "api#89", path: "internal/cache.go", line: 12, endLine: 14, side: "new" })).toMatchObject({ kind: "lines", text: "cache.go:12–14", title: "internal/cache.go:12–14" });
    expect(linesChip("cache.go", 3, 3, "old")).toMatchObject({ text: "cache.go:3", title: "cache.go:3 (removed lines)" });
  });
  it("is none for an ask of the whole", () => {
    expect(anchorChip({ pr: "api#89" })).toBeUndefined();
    expect(anchorChip(undefined)).toBeUndefined();
  });
});

describe("placed", () => {
  const at = (s: number) => new Date(Date.UTC(2026, 10, 15, 9, 0, s)).toISOString().replace(".000", "");
  const said = (n: number, by: string, s: number) => ({ n, by, at: at(s), body: "x" });
  const proposal = (n: number, s: number, kind: TalkProposal["kind"] = "comment"): TalkProposal => ({ n, kind, by: "agent", at: at(s), state: "open" });
  const t = talk({
    turns: [said(1, "you", 0), said(2, "agent", 10), said(3, "you", 20), said(4, "agent", 30)],
    proposals: [proposal(1, 5), proposal(2, 25), proposal(3, 28, "decision"), proposal(4, 40)],
  });
  it("puts each under the reply it came with, and what's since after", () => {
    const { under, after } = placed(t);
    expect(under.get(2)?.map((p) => p.n)).toEqual([1]);
    expect(under.get(4)?.map((p) => p.n)).toEqual([2, 3]);
    expect(after.map((p) => p.n)).toEqual([4]);
  });
  it("places only those asked for: the card's are review comments", () => {
    expect(placed(t, inCard).under.get(4)?.map((p) => p.n)).toEqual([2]);
  });
});
