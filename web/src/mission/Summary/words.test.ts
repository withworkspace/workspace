import { describe, expect, it } from "vitest";
import type { CrewDetail, MissionAct, MissionCard } from "../../api";
import { actsOf, actsShape, actsWide, isNotice, pickName, saidLately } from "./words";

const act = (a: Partial<MissionAct>): MissionAct => ({ do: "accept", label: "", ...a }) as MissionAct;
const crew = (c: Partial<CrewDetail>): CrewDetail => ({ id: "c1", ...c }) as CrewDetail;

describe("isNotice", () => {
  it("is a harness's warning", () => {
    expect(isNotice("orchestrator: warning: this sign-in ends between 4 and 5")).toBe(true);
    expect(isNotice("orchestrator: started c2 on step 3")).toBe(false);
  });
});

describe("saidLately", () => {
  it("is crew there, who've said something, not at work", () => {
    const d = {
      crew: [crew({ id: "c1", lastReply: "done" }), crew({ id: "c2", lastReply: "on it", activity: "working" }), crew({ id: "c3", lastReply: "bye", ended: true }), crew({ id: "c4" })],
    };
    expect(saidLately(d).map((c) => c.id)).toEqual(["c1"]);
  });
});

describe("pickName", () => {
  it("names a harness, with its model", () => {
    expect(pickName({ harness: "claude", model: "opus" })).toBe("claude:opus");
    expect(pickName({ harness: "codex" })).toBe("codex");
  });
});

describe("a card's acts", () => {
  const c = { primary: act({ do: "answer" }), more: [act({ do: "dismiss" }), act({ do: "log" })] } as MissionCard;
  it("leave what only the lead offers to the lead", () => {
    expect(actsOf(c, true).map((a) => a.do)).toEqual(["answer", "dismiss", "log"]);
    expect(actsOf(c, false).map((a) => a.do)).toEqual(["answer", "log"]);
  });
  it("are acts, unless the first is to crew", () => expect(actsShape({ crew: [] }, c, true)).toBe("acts"));
  const to = (verb: string) => ({ primary: act({ do: verb, target: "c1" }) }) as MissionCard;
  it.each([
    ["permission", "permission", {}, "prompt"],
    ["reply", "question", {}, "prompt"],
    ["reply", "idle", { status: "waiting" }, "reply"],
    ["reply", "idle", { status: "asking" }, "crew"],
    ["permission", "idle", {}, "crew"],
  ] as const)("%s to crew that's %s", (verb, activity, more, shape) => expect(actsShape({ crew: [crew({ activity, ...more } as Partial<CrewDetail>)] }, to(verb), true)).toBe(shape));
  it("are nothing when the crew have gone", () => expect(actsShape({ crew: [crew({ ended: true })] }, to("reply"), true)).toBe("none"));
  it("take the lead's width but for a way to crew", () => {
    expect(actsWide("acts")).toBe(true);
    expect(actsWide("crew")).toBe(false);
    expect(actsWide("none")).toBe(false);
  });
});
