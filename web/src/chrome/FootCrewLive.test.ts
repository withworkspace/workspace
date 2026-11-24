import { describe, expect, it } from "vitest";
import type { Transcript, TranscriptEntry } from "../api";
import { crewDoing } from "./FootCrewLive";

const of = (...entries: TranscriptEntry[]): Transcript => ({ harness: "claude", available: true, entries, version: 1 });

describe("crewDoing", () => {
  it("is its latest step, as a run going now says it", () => {
    expect(crewDoing(of({ by: "agent", text: "Looking." }, { by: "call", tool: "Edit", input: "", edit: { path: "web/src/App.tsx" } as TranscriptEntry["edit"] }), [])).toBe("Editing App.tsx");
    expect(crewDoing(of({ by: "call", tool: "Read", input: "internal/web/views.go", pending: true }), [])).toBe("Reading views.go");
  });

  it("is thinking when what's latest is something said", () => {
    expect(crewDoing(of({ by: "call", tool: "Bash", input: "ls" }, { by: "agent", text: "Done that." }), [])).toBe("Thinking");
  });

  it("is nothing until there's a session to read", () => {
    expect(crewDoing(undefined, [])).toBeUndefined();
    expect(crewDoing({ harness: "claude", available: false, entries: [], version: 0 }, [])).toBeUndefined();
    expect(crewDoing(of(), [])).toBeUndefined();
  });
});
