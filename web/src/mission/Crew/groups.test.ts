import { describe, expect, it } from "vitest";
import type { Transcript, TranscriptEntry } from "../../api";
import { latestPlan, pendingCall, transcriptGroups } from "./groups";

const t = (entries: TranscriptEntry[], earlier?: number): Transcript => ({ harness: "claude", available: true, version: 1, entries, earlier });
const call = (tool: string, more: Partial<TranscriptEntry> = {}): TranscriptEntry => ({ by: "call", tool, input: "", ...more });

describe("transcriptGroups", () => {
  it("makes a run of steps one turn, keyed by where it starts", () => {
    const g = transcriptGroups(t([{ by: "agent", text: "Reading." }, call("Read"), call("Grep"), { by: "agent", text: "Done." }, call("Bash")]));
    expect(g.map((x) => x.kind)).toEqual(["agent", "calls", "agent", "calls"]);
    expect(g[1]).toMatchObject({ at: 1, calls: [{ tool: "Read" }, { tool: "Grep" }] });
  });

  it("tells ws's brief from what you sent, only at the start of the session", () => {
    const brief = "You are crew on the mission \"Greet\" (greet), as c1.";
    expect(transcriptGroups(t([{ by: "user", text: brief }]))[0].kind).toBe("brief");
    expect(transcriptGroups(t([{ by: "user", text: brief }], 12))[0].kind).toBe("user");
    expect(transcriptGroups(t([{ by: "user", text: "Keep main.go's call as it is." }]))[0].kind).toBe("user");
  });

  it("makes a harness's notice a notice, without its brackets", () => {
    const g = transcriptGroups(t([{ by: "user", text: " [Request interrupted by user] " }, { by: "user", text: "<command-name>/clear</command-name>" }]));
    expect(g).toEqual([
      { kind: "notice", at: 0, text: "Request interrupted by user" },
      { kind: "notice", at: 1, text: "<command-name>/clear</command-name>" },
    ]);
  });
});

describe("pendingCall", () => {
  it("is the last step with nothing back yet", () => {
    expect(pendingCall(t([call("Read", { pending: true }), call("Bash", { pending: true }), { by: "agent", text: "…" }]))?.tool).toBe("Bash");
    expect(pendingCall(t([call("Read", { result: "x" })]))).toBeUndefined();
    expect(pendingCall(undefined)).toBeUndefined();
  });
});

describe("latestPlan", () => {
  it("is the todo list crew last wrote", () => {
    const first = [{ content: "a", status: "pending" as const }];
    const last = [{ content: "a", status: "completed" as const }];
    expect(latestPlan(t([call("TodoWrite", { todos: first }), call("Read"), call("TodoWrite", { todos: last })]))).toBe(last);
    expect(latestPlan(t([call("Read")]))).toBeUndefined();
  });
});
