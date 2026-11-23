import { describe, expect, it } from "vitest";
import type { TalkProposal, TalkTurn } from "../api";
import { addedVersion, afterSettling, artifactNext, artifactWhere, docLead, kindName, made, settled, spoken, staleWords, startsMission } from "./proposals";

// A proposed document's card names its kind in a sentence ("Add plan") and
// leads with what it starts with, under its title.
describe("kindName", () => {
  it("lowers a word's first letter", () => {
    expect(kindName("Plan")).toBe("plan");
    expect(kindName("Build proposal")).toBe("build proposal");
  });
  it("leaves an acronym as it is", () => {
    expect(kindName("ADR")).toBe("ADR");
    expect(kindName("BP")).toBe("BP");
  });
});

describe("docLead", () => {
  it("is the first paragraph that isn't a heading", () => {
    expect(docLead("# Todos\n\nWhat you've noticed,\nkept.\n\n## More\n\nLater.")).toBe("What you've noticed, kept.");
  });
  it("skips front matter's rule and is clipped", () => {
    expect(docLead("---\n\n# T\n\n" + "x".repeat(400))).toHaveLength(280);
  });
  it("is nothing for a document that's only headings", () => {
    expect(docLead("# Todos\n\n## Later\n")).toBe("");
  });
});

// A turn ws wrote for a card you settled reads as a marker of what became
// of it, from the proposal as it stands: never as your words.
describe("settled", () => {
  const turn = (n: number, body: string, quiet = true): TalkTurn => ({ n: 10, by: "you", at: "2026-11-16T14:20:00Z", body, origin: `proposal:${n}`, quiet });
  const prop = (n: number, kind: TalkProposal["kind"], state: TalkProposal["state"], more: Partial<TalkProposal> = {}): TalkProposal => ({ n, kind, by: "agent", at: "2026-11-16T14:19:00Z", state, ...more });

  it("is nothing for a turn you typed", () => {
    expect(settled({ n: 1, by: "you", at: "", body: "ok, accepted" })).toBeUndefined();
  });
  it("names what each kind accepted made, and where it is", () => {
    const ps = [
      prop(1, "mission", "accepted", { title: "Token cache", result: "token-cache" }),
      prop(2, "decision", "accepted", { what: "Round half to even.", result: "cache:d1" }),
      prop(3, "document", "accepted", { title: "How loud", result: "records:adr/0008-how-loud.md" }),
      prop(4, "brief", "accepted", { brief: "## Goal", plan: "## Z" }),
      prop(5, "plan", "accepted", { plan: "## Z", was: "## Y" }),
      prop(6, "dispatch", "accepted", { text: "Fix it", result: "c3" }),
      prop(7, "comment", "accepted", { pr: "api#12", path: "cache.go", line: 12, result: "n1" }),
    ];
    const marker = (n: number) => settled(turn(n, "Accepted proposal …"), ps);
    expect(marker(1)).toEqual({ accepted: true, what: "mission token-cache", to: "/missions/token-cache", edits: undefined });
    expect(marker(2)?.what).toBe("decision “Round half to even”");
    expect(marker(3)).toMatchObject({ what: "document records:adr/0008-how-loud.md", to: "/documents/records/adr/0008-how-loud.md" });
    expect(marker(4)?.what).toBe("the brief and its plan");
    expect(marker(5)?.what).toBe("the edit to the plan");
    expect(marker(6)?.what).toBe("crew c3");
    expect(marker(7)?.what).toBe("review comment on cache.go:12");
  });
  it("names an artifact, or a version of one, and leads to the version it made", () => {
    const ps = [
      prop(1, "artifact", "accepted", { title: "Horizon glow", result: "a1.v1" }),
      prop(2, "artifact", "accepted", { to: "a1", on: 1, result: "a1.v2" }),
      prop(3, "artifact", "declined", { to: "a1", on: 2 }),
    ];
    expect(settled(turn(1, "Accepted proposal 1, …"), ps)).toMatchObject({ what: "artifact “Horizon glow”", to: "/artifacts/a1/v1" });
    expect(settled(turn(2, "Accepted proposal 2, …"), ps)).toMatchObject({ what: "v2 of a1", to: "/artifacts/a1/v2" });
    expect(settled(turn(3, "Declined proposal 3, …"), ps)).toEqual({ accepted: false, what: "a version of a1, on v2", to: undefined, edits: undefined });
  });
  it("names an edit to a document, and leads to where it was saved", () => {
    const ps = [
      prop(1, "document", "accepted", { to: "records:adr/0007-one-binary.md", was: "# One binary\n", text: "# One binary\n\nAnd why.", result: "records:adr/0007-one-binary.md" }),
      prop(2, "document", "declined", { to: "api:notes/x.md", was: "# X\n", text: "# X\n\nMore." }),
    ];
    expect(settled(turn(1, "Accepted proposal 1, …"), ps)).toMatchObject({ what: "edit to records:adr/0007-one-binary.md", to: "/documents/records/adr/0007-one-binary.md" });
    expect(settled(turn(2, "Declined proposal 2, …", false), ps)).toEqual({ accepted: false, what: "edit to api:notes/x.md", to: undefined, edits: undefined });
  });
  it("says where a replan was asked, or what was declined", () => {
    const ps = [
      prop(1, "replan", "accepted", { text: "Add a step for the dropdown.", result: "t2" }),
      prop(2, "replan", "declined", { text: "Drop the browser check." }),
    ];
    expect(settled(turn(1, "Accepted proposal 1, …"), ps)).toEqual({ accepted: true, what: "replan, asked of the planner in t2", edits: undefined });
    expect(settled(turn(2, "Declined proposal 2, …", false), ps)).toEqual({ accepted: false, what: "replan “Drop the browser check”", edits: undefined });
  });
  it("names what was declined as it was proposed", () => {
    const m = settled(turn(1, "Declined proposal 1, the mission “Token cache”.", false), [prop(1, "mission", "declined", { title: "Token cache" })]);
    expect(m).toEqual({ accepted: false, what: "mission “Token cache”", edits: undefined });
  });
  it("keeps what you edited, past the first line, for the agent's text", () => {
    const body = "Accepted proposal 2, the decision “Round half up.”: recorded as d1 on mission cache.\n\nEdited before accepting.\n\nThe decision: “Round half up.” (proposed: “Round half to even.”).";
    const m = settled(turn(2, body), [prop(2, "decision", "accepted", { what: "Round half to even.", result: "cache:d1" })]);
    expect(m?.edits).toBe("Edited before accepting.\n\nThe decision: “Round half up.” (proposed: “Round half to even.”).");
  });
  it("falls back on the turn's first line without its proposal", () => {
    expect(settled(turn(9, "Declined proposal 9, the edit to the brief."))).toEqual({ accepted: false, what: "proposal 9, the edit to the brief", edits: undefined });
  });
});

// Whose turn it is skips the quiet ones, as ws does.
describe("spoken", () => {
  const t = (n: number, by: string, quiet?: boolean): TalkTurn => ({ n, by, at: "", body: "", quiet });
  it("is the latest turn but for quiet ones", () => {
    expect(spoken([t(1, "you"), t(2, "agent"), t(3, "you", true), t(4, "you", true)])?.n).toBe(2);
    expect(spoken([t(1, "you"), t(2, "agent"), t(3, "you")])?.n).toBe(3);
    expect(spoken([])).toBeUndefined();
  });
});

// A proposed version of an artifact is added one past its latest, and its
// card says when versions were added since it was proposed (w9's v2 added
// by crew while the conversation proposed a revision of v1).
describe("artifactNext", () => {
  it("is v1 for a new artifact", () => {
    expect(artifactNext({})).toBe(1);
  });
  it("is one past the latest, as it is now", () => {
    expect(artifactNext({ to: "a1", on: 1, latest: 1 }, { latest: { n: 1 } })).toBe(2);
    expect(artifactNext({ to: "a1", on: 1, latest: 1 }, { latest: { n: 3 } })).toBe(4);
  });
  it("is one past the latest it was proposed on, with the artifact not read yet", () => {
    expect(artifactNext({ to: "a1", on: 1, latest: 2 })).toBe(3);
  });
});

describe("staleWords", () => {
  it("is nothing while no version's been added since", () => {
    expect(staleWords({ to: "a1", on: 1, latest: 1 }, { latest: { n: 1 } })).toBe("");
    expect(staleWords({}, { latest: { n: 4 } })).toBe("");
  });
  it("names the version added since, on the one it was proposed on", () => {
    expect(staleWords({ to: "a1", on: 1, latest: 1 }, { latest: { n: 2 } })).toBe("Proposed on v1; v2 has been added since");
  });
  it("names each, when there are more", () => {
    expect(staleWords({ to: "a1", on: 1, latest: 2 }, { latest: { n: 5 } })).toBe("Proposed on v1; v3, v4 and v5 have been added since");
  });
});

describe("addedVersion", () => {
  it("reads the version accepting it made", () => {
    expect(addedVersion("a3.v2")).toEqual({ id: "a3", n: 2 });
  });
  it("is nothing for anything else", () => {
    expect(addedVersion("a3")).toBeUndefined();
    expect(addedVersion(undefined)).toBeUndefined();
    expect(addedVersion("records:docs/x.md")).toBeUndefined();
  });
});

describe("artifactWhere", () => {
  it("says a new one goes where the workspace keeps new ones", () => {
    expect(artifactWhere({ path: ".ws/artifacts/2026-11-16-glow/" })).toBe("Adds .ws/artifacts/2026-11-16-glow/, on this machine");
    expect(artifactWhere({ path: "records:artifacts/2026-11-16-glow/v1.html" })).toBe("Adds records:artifacts/2026-11-16-glow/v1.html, in a commit on records");
    expect(artifactWhere({})).toBe("");
  });

  it("says a version goes where its artifact is", () => {
    expect(artifactWhere({ to: "a1", on: 1, latest: 1 }, { latest: { n: 1 }, place: "records" })).toBe("Adds v2 to a1, in a commit on records");
    expect(artifactWhere({ to: "a1", on: 1, latest: 1 }, { latest: { n: 2 }, place: ".ws" })).toBe("Adds v3 to a1, on this machine");
    expect(artifactWhere({ to: "a1", on: 1, latest: 1 })).toBe("Adds v2 to a1");
  });
});

// Beside a conversation: what its proposals made, and what's waiting to
// be confirmed, each with where it is now.
describe("made", () => {
  const prop = (n: number, kind: TalkProposal["kind"], state: TalkProposal["state"], more: Partial<TalkProposal> = {}): TalkProposal => ({ n, kind, by: "agent", at: "2026-11-16T14:19:00Z", state, ...more });

  it("lists what was accepted with where it is, and what's open without", () => {
    const ps = [
      prop(1, "artifact", "accepted", { title: "Mission chrome", result: "a2.v1" }),
      prop(2, "document", "accepted", { title: "Notes on chrome", result: "records:notes/chrome.md" }),
      prop(3, "mission", "accepted", { title: "Token cache", result: "token-cache" }),
      prop(4, "decision", "accepted", { what: "Review stays a page of its own.\nMore.", result: "token-cache:d3" }),
      prop(5, "artifact", "open", { to: "a2", on: 1 }),
    ];
    expect(made(ps, "", (id) => (id === "a2" ? "Mission chrome" : undefined))).toEqual([
      { n: 1, kind: "artifact", name: "Mission chrome", tag: "a2.v1", to: "/artifacts/a2/v1", open: false },
      { n: 2, kind: "document", name: "Notes on chrome", tag: undefined, to: "/documents/records/notes/chrome.md", open: false },
      { n: 3, kind: "mission", name: "Token cache", tag: undefined, to: "/missions/token-cache", open: false },
      { n: 4, kind: "decision", name: "Review stays a page of its own", tag: "d3", to: "/missions/token-cache", open: false },
      { n: 5, kind: "artifact", name: "Mission chrome", tag: "a version of a2", to: undefined, open: true },
    ]);
  });
  it("leaves out what was declined or superseded", () => {
    expect(made([prop(1, "mission", "declined", { title: "A" }), prop(2, "mission", "superseded", { title: "B", result: "3" })])).toEqual([]);
  });
  it("keeps one brief and one plan: the latest accepted, and any still open", () => {
    const ps = [prop(1, "brief", "accepted", { brief: "a" }), prop(2, "plan", "accepted", { plan: "p" }), prop(3, "brief", "accepted", { brief: "b", plan: "p" }), prop(4, "brief", "open", { brief: "c" })];
    expect(made(ps, "token-cache").map((m) => [m.n, m.name, m.to])).toEqual([
      [2, "The plan", "/missions/token-cache/brief"],
      [3, "The brief and its plan", "/missions/token-cache/brief"],
      [4, "The brief", undefined],
    ]);
  });
  it("says a mission saved without a brief is an idea, and leads crew and a replan to the mission", () => {
    const ps = [prop(1, "mission", "accepted", { title: "Later" }), prop(2, "dispatch", "accepted", { text: "Fix the test.", result: "c2" }), prop(3, "replan", "accepted", { text: "Split step 2", result: "t4" })];
    expect(made(ps, "token-cache")).toEqual([
      { n: 1, kind: "mission", name: "Later", tag: "idea", to: undefined, open: false },
      { n: 2, kind: "dispatch", name: "Fix the test", tag: "c2", to: "/missions/token-cache", open: false },
      { n: 3, kind: "replan", name: "Split step 2", tag: "t4", to: "/missions/token-cache/conversations/t4", open: false },
    ]);
  });
});

// Accepting a brief that starts its mission goes to the mission's summary,
// wherever it's accepted; anything else stays, or follows a conversation
// that's moved.
describe("afterSettling", () => {
  const prop = (x: Partial<TalkProposal>): TalkProposal => ({ n: 1, kind: "brief", state: "open", at: "2026-11-23T10:00:00Z", ...x }) as TalkProposal;
  const briefing = prop({ title: "Archive old invoices", capsules: ["api/archive"] });
  const edit = prop({ brief: "A new goal." });

  it("knows a brief that starts its mission from an edit to one", () => {
    expect(startsMission(briefing)).toBe(true);
    expect(startsMission(prop({ slug: "archive-old-invoices" }))).toBe(true);
    expect(startsMission(edit)).toBe(false);
    expect(startsMission(prop({ kind: "plan", title: "A plan" }))).toBe(false);
  });
  it("goes to the mission's summary once its brief is confirmed", () => {
    expect(afterSettling(briefing, "accept", "archive-invoices")).toBe("/missions/archive-invoices");
  });
  it("goes to the mission as it's named now, when confirming renamed it", () => {
    expect(afterSettling(briefing, "accept", "archive-invoices", "/missions/archive-old-invoices/conversations/t1")).toBe("/missions/archive-old-invoices");
  });
  it("stays for a declined draft, and for an edit to a brief under way", () => {
    expect(afterSettling(briefing, "decline", "archive-invoices")).toBeUndefined();
    expect(afterSettling(edit, "accept", "archive-invoices")).toBeUndefined();
    expect(afterSettling(undefined, "accept", "archive-invoices")).toBeUndefined();
  });
  it("follows a conversation that moved, for anything else", () => {
    expect(afterSettling(edit, "accept", "old", "/missions/new/conversations/t1")).toBe("/missions/new/conversations/t1");
  });
  it("stays in a workspace's conversation", () => {
    expect(afterSettling(briefing, "accept", "")).toBeUndefined();
  });
});
