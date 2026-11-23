import { describe, expect, it } from "vitest";
import {
  arrived,
  dealtWithAt,
  type Need,
  needSays,
  needStatus,
  waitingOf,
} from "./need";
import { needs } from "./samples";

const need = (n: Partial<Need>): Need => ({
  key: "k",
  of: "mission",
  ref: "m",
  kind: "step-review",
  title: "A mission",
  to: "/missions/m",
  ...n,
});

describe("how a need stands", () => {
  it("is as its mission's headline's kind has it, with the headline's own word", () => {
    expect(
      needStatus(need({ kind: "crew-asks", label: "? asks" })),
    ).toMatchObject({ icon: "question", tone: "attention", words: "Asks" });
    expect(needStatus(need({ kind: "merge", label: "✓ merge" }))).toMatchObject(
      { icon: "pr", tone: "attention" },
    );
  });

  it("is your turn, for a conversation", () => {
    expect(
      needStatus(need({ of: "conversation", kind: "your-turn" })),
    ).toMatchObject({ icon: "reply", tone: "attention", words: "Your turn" });
  });

  it("asks for you even of a kind it hasn't met", () => {
    expect(needStatus(need({ kind: "something-new" })).tone).toBe("attention");
  });
});

describe("what a need says", () => {
  it("is its sentence in plain words", () => {
    expect(
      needSays(need({ text: "Step 3 is done: **Fix it.** `go test` passes" })),
    ).toBe("Step 3 is done: Fix it. go test passes");
    expect(needSays(need({}))).toBe("");
  });
});

describe("the needs, for the places' counts and the desktop", () => {
  it("says which are missions and which conversations", () => {
    const waiting = waitingOf([
      need({ key: "a" }),
      need({
        key: "b",
        of: "conversation",
        kind: "your-turn",
        to: "/conversations/w1",
        text: "The agent replied: your turn.",
      }),
    ]);
    expect(waiting.map((w) => w.kind)).toEqual(["mission", "talk"]);
    expect(waiting[1]).toMatchObject({
      key: "b",
      path: "/conversations/w1",
      body: "The agent replied: your turn.",
    });
  });

  it("counts a conversation about a mission with the missions", () => {
    const [w] = waitingOf([
      need({
        of: "conversation",
        kind: "your-turn",
        in: "dark",
        to: "/missions/dark/conversations/t1",
      }),
    ]);
    expect(w.kind).toBe("mission");
  });
});

describe("the stories' needs", () => {
  it("are Sorrel's missions in your court, then the conversations that are your turn", () => {
    expect(needs.map((n) => n.to)).toEqual([
      "/missions/invoice-totals-off-by-a-penny",
      "/missions/export-invoices-as-csv",
      "/missions/retry-failed-webhooks-with-backoff",
      "/missions/rate-limit-the-public-api",
      "/conversations/w2",
      "/conversations/w1",
    ]);
  });
});

describe("a need arriving", () => {
  it("is one that's here now and wasn't before", () => {
    expect(arrived(["a", "b"], ["c", "a", "b"])).toEqual(["c"]);
    expect(arrived(["a", "b"], ["b"])).toEqual([]);
  });

  it("isn't news when the page has only just opened", () => {
    expect(arrived(undefined, ["a", "b"])).toEqual([]);
  });

  it("is again, when the same thing needs you for something else", () => {
    expect(
      arrived(["mission|dark|step-review||1"], ["mission|dark|crew-asks|c1|2"]),
    ).toEqual(["mission|dark|crew-asks|c1|2"]);
  });
});

describe("where a need is dealt with", () => {
  const dark = need({ to: "/missions/dark" });

  it("is its own page and the pages under it", () => {
    expect(dealtWithAt(dark, "/missions/dark")).toBe(true);
    expect(dealtWithAt(dark, "/missions/dark/review/web/dark-mode")).toBe(true);
  });

  it("isn't another thing whose name starts the same", () => {
    expect(dealtWithAt(dark, "/missions/dark-mode")).toBe(false);
    expect(dealtWithAt(dark, "/")).toBe(false);
  });
});
