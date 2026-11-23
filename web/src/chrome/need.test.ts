import { describe, expect, it } from "vitest";
import {
  arrived,
  dealtWithAt,
  type Need,
  needSays,
  needStatus,
  toReview,
  waitingOf,
} from "./need";
import { placesOf } from "../shell/places";
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

  it("is your review, for a pull request that asks you by name", () => {
    expect(
      needStatus(need({ of: "pull", kind: "your-review", ref: "api#89" })),
    ).toMatchObject({ icon: "pr", tone: "attention", words: "Your review" });
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

describe("a pull request that asks for your review", () => {
  const pull = need({
    key: "pull|api#89|",
    of: "pull",
    ref: "api#89",
    kind: "your-review",
    to: "/reviews/api/89",
    text: "api#89, from alice, needs your review.",
  });

  it("is told apart from what's yours", () => {
    expect(toReview(pull)).toBe(true);
    expect(toReview(need({}))).toBe(false);
  });

  it("counts at neither Missions nor Conversations, and is still told to the desktop", () => {
    const [w] = waitingOf([pull]);
    expect(w).toMatchObject({
      kind: "review",
      path: "/reviews/api/89",
      body: "api#89, from alice, needs your review.",
    });
    const places = placesOf(undefined, waitingOf([need({}), pull]));
    expect(places.find((p) => p.id === "missions")?.count).toBe(1);
    expect(places.find((p) => p.id === "conversations")?.count).toBe(0);
  });

  it("is dealt with on its review", () => {
    expect(dealtWithAt(pull, "/reviews/api/89")).toBe(true);
    expect(dealtWithAt(pull, "/reviews")).toBe(false);
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
