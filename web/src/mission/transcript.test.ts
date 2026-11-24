import { describe, expect, it } from "vitest";
import type { Transcript, TranscriptEntry } from "../api";
import type { TranscriptOp } from "../wire.gen";
import { appliedTranscript, shownEntries } from "./transcript";

const said = (text: string): TranscriptEntry => ({ by: "agent", text });
const call = (pending: boolean): TranscriptEntry => ({ by: "call", tool: "Bash", input: "ls", pending, result: pending ? undefined : "a.go" });
const held: Transcript = { harness: "claude", available: true, entries: [said("Looking."), call(true)], version: 100 };
const op = (o: Partial<TranscriptOp>): TranscriptOp => ({ version: 200, from: 2, entries: [], harness: "claude", available: true, ...o });

describe("appliedTranscript", () => {
  it("adds what's said after what's held, and answers a call in hand", () => {
    const d = appliedTranscript(held, op({ from: 1, entries: [call(false), said("Done.")] })) as Transcript;
    expect(d.entries.map((e) => e.by)).toEqual(["agent", "call", "agent"]);
    expect(d.entries[1].pending).toBeFalsy();
    expect(d.version).toBe(200);
  });

  it("drops a change it holds already, and reads it whole when it's missed one, or it's before what it holds", () => {
    expect(appliedTranscript(held, op({ version: 100 }))).toBeUndefined();
    expect(appliedTranscript(held, op({ from: 5 }))).toBe("read");
    expect(appliedTranscript({ ...held, earlier: 10 }, op({ from: 3 }))).toBe("read");
    expect(appliedTranscript(undefined, op({}))).toBe("read");
  });

  it("takes a session that's started since, whole", () => {
    const none: Transcript = { harness: "claude", available: false, why: "it hasn't started", entries: [], version: 0 };
    const d = appliedTranscript(none, op({ from: 0, entries: [said("Hi.")], version: 40 })) as Transcript;
    expect(d.available).toBe(true);
    expect(d.why).toBeUndefined();
    expect(d.entries).toHaveLength(1);
  });

  it("keeps the latest it's shown, counting those it leaves out", () => {
    const full: Transcript = { ...held, entries: Array.from({ length: shownEntries }, (_, i) => said(`${i}`)), earlier: 7 };
    const d = appliedTranscript(full, op({ from: 7 + shownEntries, entries: [said("new"), said("newer")] })) as Transcript;
    expect(d.entries).toHaveLength(shownEntries);
    expect(d.earlier).toBe(9);
    expect(d.entries[d.entries.length - 1].text).toBe("newer");
  });
});
