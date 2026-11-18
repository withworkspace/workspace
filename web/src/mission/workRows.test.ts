import { describe, expect, it } from "vitest";
import type { Artifact, CapsuleDetail, ChangedFile, CrewDetail, MissionDetail, Stage } from "../api";
import { capsuleState, crewElsewhere, fileGroups, outputHome, stepCapsules, work } from "./workRows";

const cap = (repo: string, name: string, more: Partial<CapsuleDetail> = {}): CapsuleDetail => ({ repo, name, path: `/w/repos/${repo}/${name}`, exists: true, ...more });
const stage = (n: number, type: Stage["type"], who: Stage["who"], more: Partial<Stage> = {}): Stage => ({ n, title: `Step ${n}`, text: "", state: "todo", type, who, ...more });
const crew = (id: string, capsule: string, more: Partial<CrewDetail> = {}): CrewDetail => ({ id, harness: "claude", capsule, started: "2026-11-17T18:00:00Z", ended: true, ...more });
const art = (ref: string, kind: string, added: string): Artifact => ({ ref, kind, state: "private", added, readable: true });

// A mission across three repos: the design landed from records, ui-lib's
// pull request merged, ws is at the step that opens its pull request, and
// the plugin kit's lifted for later.
function mission(more: Partial<MissionDetail> = {}): MissionDetail {
  return {
    slug: "m",
    stages: [
      stage(1, "Work", "crew", { state: "accepted", in: "records", crew: ["c1"] }),
      stage(2, "Review", "you", { state: "accepted" }),
      stage(3, "Land", "you", { state: "accepted" }),
      stage(4, "Work", "crew", { state: "accepted", in: "ui-lib", crew: ["c2"] }),
      stage(5, "Open Pull Request", "ws", { state: "accepted", in: "ui-lib", opensPR: true }),
      stage(6, "Decide", "you", { state: "accepted" }),
      stage(7, "Work", "crew", { state: "accepted", in: "workspace", crew: ["c3"] }),
      stage(8, "Verify", "crew", { state: "accepted", in: "workspace", crew: ["c4"] }),
      stage(9, "Review", "you", { state: "accepted" }),
      stage(10, "Open Pull Request", "ws", { state: "current", in: "workspace", opensPR: true }),
      stage(11, "Work", "crew", { in: "plugin" }),
      stage(12, "Wait", "outside"),
    ],
    capsules: [
      cap("records", "m", { exists: false, direct: true, landed: "2026-11-17T18:22:00Z" }),
      cap("workspace", "m", { ahead: 4 }),
      cap("plugin", "m"),
      cap("ui-lib", "m", { pr: { number: 2, title: "ui-lib", url: "https://example.com/2", merged: true } }),
    ],
    crew: [
      crew("c1", "records/m", { handoff: "mission:handoffs/c1.md" }),
      crew("c2", "ui-lib/m", { handoff: "mission:handoffs/c2.md" }),
      crew("c3", "workspace/m", { handoff: "mission:handoffs/c3.md" }),
      crew("c4", "workspace/m", { ended: false, activity: "waiting", status: "done" }),
    ],
    artifacts: [
      art("mission:handoffs/c1.md", "handoff", "2026-11-17T18:20:00Z"),
      art("records/m:plans/plan.md", "doc", "2026-11-17T18:21:00Z"),
      art("mission:handoffs/c2.md", "handoff", "2026-11-17T19:25:00Z"),
      art("mission:handoffs/c3.md", "handoff", "2026-11-17T22:19:00Z"),
      art("mission:note.md", "note", "2026-11-17T22:30:00Z"),
    ],
    bundles: [],
    ...more,
  } as unknown as MissionDetail;
}

describe("stepCapsules", () => {
  it("puts a step where its crew were, where the plan says, or where the step before ran", () => {
    const at = stepCapsules(mission());
    expect(at.get(1)).toEqual(["records/m"]); // its crew's capsule
    expect(at.get(2)).toEqual(["records/m"]); // a review of the step before
    expect(at.get(3)).toEqual(["records/m"]);
    expect(at.get(5)).toEqual(["ui-lib/m"]); // the plan names the repo
    expect(at.get(11)).toEqual(["plugin/m"]);
  });

  it("leaves a step that's of no capsule out, and doesn't lose the place for the one after", () => {
    const at = stepCapsules(mission());
    expect(at.has(6)).toBe(false); // a decision
    expect(at.has(12)).toBe(false); // a wait
    expect(at.get(9)).toEqual(["workspace/m"]);
  });

  it("takes a repo with two capsules to the one that's still here", () => {
    const d = mission({ capsules: [cap("records", "old", { exists: false, landed: "2026-11-17T18:22:00Z" }), cap("records", "new")], stages: [stage(1, "Work", "crew", { in: "records" })], crew: [] });
    expect(stepCapsules(d).get(1)).toEqual(["records/new"]);
  });

  it("puts a step that runs in each of several repos in each", () => {
    const d = mission({ stages: [stage(1, "Work", "crew", { runs: [{ repo: "workspace" }, { repo: "plugin", capsule: "plugin/m" }] })], crew: [] });
    expect(stepCapsules(d).get(1)).toEqual(["workspace/m", "plugin/m"]);
  });
});

describe("work", () => {
  it("lists capsules in hand, then up next, then done, and opens on the one the mission's at", () => {
    const w = work(mission());
    expect(w.hand.map((x) => x.id)).toEqual(["workspace/m"]);
    expect(w.next.map((x) => x.id)).toEqual(["plugin/m"]);
    expect(w.done.map((x) => x.id)).toEqual(["records/m", "ui-lib/m"]);
    expect(w.opensOn).toBe("c:workspace/m");
    expect(w.keys.slice(0, 4)).toEqual(["c:workspace/m", "c:plugin/m", "c:records/m", "c:ui-lib/m"]);
  });

  it("says where each stands", () => {
    const by = Object.fromEntries(work(mission()).capsules.map((x) => [x.id, x.state.text]));
    expect(by).toEqual({ "workspace/m": "Pull request to open", "plugin/m": "From step 11", "records/m": expect.stringMatching(/^Landed /), "ui-lib/m": "#2 merged" });
  });

  it("gives a capsule its steps, its crew, who's in it now and what was made there", () => {
    const ws = work(mission()).capsules.find((x) => x.id === "workspace/m")!;
    expect(ws.steps.map((s) => s.n)).toEqual([7, 8, 9, 10]);
    expect(ws.now?.n).toBe(10);
    expect(ws.crew.map((c) => c.id)).toEqual(["c3", "c4"]);
    expect(ws.live?.id).toBe("c4");
    expect(ws.made.map((a) => a.ref)).toEqual(["mission:handoffs/c3.md"]);
    const records = work(mission()).capsules.find((x) => x.id === "records/m")!;
    expect(records.made.map((a) => a.ref)).toEqual(["mission:handoffs/c1.md", "records/m:plans/plan.md"]);
  });

  it("keeps a merged capsule in hand while crew are at work in it again", () => {
    const d = mission();
    d.crew.push(crew("c5", "ui-lib/m", { ended: false, activity: "working" }));
    const ui = work(d).capsules.find((x) => x.id === "ui-lib/m")!;
    expect(ui.where).toBe("hand");
    expect(ui.state).toMatchObject({ text: "c5 working", live: true });
  });

  it("lists documents and notes first, then handoffs, newest first", () => {
    expect(work(mission()).outputs.map((a) => a.ref)).toEqual(["records/m:plans/plan.md", "mission:note.md", "mission:handoffs/c3.md", "mission:handoffs/c2.md", "mission:handoffs/c1.md"]);
  });

  it("opens on an output when there's no capsule", () => {
    expect(work(mission({ capsules: [], crew: [] })).opensOn).toBe("a:records/m:plans/plan.md");
    expect(work(mission({ capsules: [], crew: [], artifacts: [] })).opensOn).toBeUndefined();
  });
});

describe("capsuleState", () => {
  it("puts crew asking before anything else", () => {
    expect(capsuleState(cap("a", "b", { pr: { number: 1, title: "", url: "" } }), [], crew("c1", "a/b", { ended: false, activity: "question" }))).toMatchObject({ text: "c1 asks", tone: "attention" });
  });

  it("says a step of yours as what it asks of you", () => {
    expect(capsuleState(cap("a", "b"), [stage(2, "Review", "you", { state: "current" })]).text).toBe("Yours to check");
    expect(capsuleState(cap("a", "b"), [stage(2, "Land", "you", { state: "current" })]).text).toBe("Yours to land");
  });

  it("says a pull request as it stands", () => {
    expect(capsuleState(cap("a", "b", { pr: { number: 7, title: "", url: "u", checks: "FAILURE" } }), [])).toMatchObject({ text: "#7 checks failing", tone: "danger", url: "u" });
    expect(capsuleState(cap("a", "b", { pr: { number: 7, title: "", url: "u" } }), []).text).toBe("#7 open");
  });
});

describe("outputHome", () => {
  it("is a handoff's crew, their capsule and the step they took", () => {
    const d = mission();
    const home = outputHome(d, work(d), d.artifacts[3]);
    expect(home.crew?.id).toBe("c3");
    expect(home.capsule?.id).toBe("workspace/m");
    expect(home.step?.n).toBe(7);
  });

  it("is nothing for a note of the mission's own", () => {
    const d = mission();
    expect(outputHome(d, work(d), d.artifacts[4])).toEqual({ crew: undefined, capsule: undefined, step: undefined });
  });
});

describe("crewElsewhere", () => {
  it("is crew in no capsule of the mission's: none at all, or one that ended alone", () => {
    const d = mission();
    d.crew.push(crew("c5", ""), crew("c6", "demo/handed-on"));
    expect(crewElsewhere(d).map((c) => c.id)).toEqual(["c5", "c6"]);
  });
});

describe("fileGroups", () => {
  const f = (path: string, additions = 1, deletions = 0): ChangedFile => ({ path, status: "modified", additions, deletions });
  const files = [f("README.md", 2, 1), f("internal/web/api.go", 5, 0), f("web/src/App.tsx", 3, 3), f("internal/cli/x.go", 1, 4)];

  it("groups by top-level folder, in the order they came, and counts each", () => {
    expect(fileGroups(files).map((g) => [g.dir, g.files.length, g.additions, g.deletions])).toEqual([["", 1, 2, 1], ["internal", 2, 6, 4], ["web", 1, 3, 3]]);
  });

  it("keeps what the filter's in, whatever its case", () => {
    expect(fileGroups(files, " API ").map((g) => g.files.map((x) => x.path))).toEqual([["internal/web/api.go"]]);
  });
});
