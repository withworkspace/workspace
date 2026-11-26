import { describe, expect, it } from "vitest";
import type { TalkDetail, TalkTurn } from "../api";
import type { ConversationOp } from "../wire.gen";
import { applied, asking, askingAt, isAsking, useTalks } from "./talks";

let n = 0;
const turn = (body: string): TalkTurn => ({ n: ++n, by: "you", at: "2026-11-24T10:00:00Z", body }) as TalkTurn;
const held: TalkDetail = { id: "w1", kind: "talk", by: "you", at: "2026-11-24T10:00:00Z", status: "open", turns: [turn("Why?")], version: 5 };
const op = (o: Partial<ConversationOp>): ConversationOp =>
  ({ version: 6, from: 1, turns: [], conversation: { id: "w1", kind: "talk", by: "you", at: "2026-11-24T10:00:00Z", status: "open", turns: null }, ...o }) as unknown as ConversationOp;

describe("applied", () => {
  it("adds the turns after those held, and takes the rest as it now is", () => {
    const got = applied(held, op({ turns: [turn("Because.")] as never, conversation: { ...op({}).conversation!, answering: true } }));
    expect(got).not.toBe("read");
    const d = got as TalkDetail;
    expect(d.turns.map((t) => t.body)).toEqual(["Why?", "Because."]);
    expect(d.answering).toBe(true);
    expect(d.version).toBe(6);
  });

  it("says again turns from where they differ", () => {
    const two = { ...held, turns: [turn("Why?"), turn("Becau")] };
    const d = applied(two, op({ from: 1, turns: [turn("Because.")] as never })) as TalkDetail;
    expect(d.turns.map((t) => t.body)).toEqual(["Why?", "Because."]);
  });

  it("drops a change it holds already, and reads it whole when it's missed one", () => {
    expect(applied(held, op({ version: 4 }))).toBeUndefined();
    expect(applied(held, op({ from: 3 }))).toBe("read");
    expect(applied(undefined, op({}))).toBe("read");
  });

  it("takes the run as it is, none when nobody's answering", () => {
    const answering = applied(held, op({ run: { calls: [], said: "Reading." } })) as TalkDetail;
    expect(answering.run?.said).toBe("Reading.");
    expect((applied(answering, op({ version: 7 })) as TalkDetail).run).toBeUndefined();
  });
});

describe("asking", () => {
  it("is held at an address of its own, which is never read from the daemon", async () => {
    const to = askingAt();
    expect(isAsking(to)).toBe(true);
    expect(isAsking("/conversations/w1")).toBe(false);
    expect(askingAt()).not.toBe(to);
    useTalks.getState().told(to, asking("Why?"));
    await useTalks.getState().load(to); // nothing to read: what's held stays
    const d = useTalks.getState().talks[to]?.detail;
    expect(d?.turns.map((t) => t.body)).toEqual(["Why?"]);
    expect(d?.answering).toBe(true);
    await expect(useTalks.getState().say(to, "And?")).rejects.toThrow();
  });
});
