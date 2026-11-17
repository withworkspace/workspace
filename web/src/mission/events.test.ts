import { describe, expect, it } from "vitest";
import type { MissionEvent } from "../api";
import { endedWords, viewEvent } from "./parts";

const ev = (e: Partial<MissionEvent>): MissionEvent => ({ time: "2026-11-16T17:28:00Z", kind: "resume_with", text: "", ...e });

describe("viewEvent: who said a message to crew", () => {
  it.each([
    ["the orchestrator typed it in", ev({ text: "told c2: Step 3's checks…", by: "orchestrator", to: "c2" }), "Orchestrator"],
    ["you told running crew", ev({ kind: "told", text: "told c2: push it", by: "you", to: "c2" }), "You"],
    ["you typed in its terminal", ev({ kind: "told", text: "typed into c2's terminal", by: "you", to: "c2" }), "You"],
    ["you resumed it with a message", ev({ text: "suggested resuming c5: carry on", by: "you", to: "c5" }), "You"],
    ["ws resumed it", ev({ text: "suggested resuming c4: Your session ended…", by: "ws", to: "c4" }), "ws"],
    // Logged before ws said who: typed in was only ever ws's or the
    // orchestrator's, never yours.
    ["typed in, before ws said", ev({ text: "told c2: Step 3's checks…", to: "c2" }), "Orchestrator"],
    ["suggested, before ws said", ev({ text: "suggested resuming c5: go on", to: "c5" }), "Orchestrator"],
  ])("%s", (_, e, who) => expect(viewEvent(e).who).toBe(who));
});

describe("how crew ended", () => {
  it("says a stop logged before ws said how, not its raw exit", () => {
    expect(viewEvent(ev({ kind: "crew_ended", text: "c5 ended (stopped: exit 129)", by: "c5" })).text).toBe("was stopped");
    expect(viewEvent(ev({ kind: "crew_ended", text: "c4 ended (exit 129)", by: "c4" })).text).toBe("ended: its terminal closed");
  });
  it.each([
    [{ how: "interrupted" as const, exit: "stopped: exit 129" }, "stopped by a daemon restart"],
    [{ how: "handed-back" as const, exit: "exit 129" }, "closed: it had handed its work back"],
    [{ how: "stopped" as const, exit: "exit 129" }, "stopped"],
    [{ exit: "exit 1" }, "stopped with an error"],
  ])("%o reads %s", (c, words) => expect(endedWords(c)).toBe(words));
});

