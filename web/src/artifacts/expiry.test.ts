import { describe, expect, it } from "vitest";
import type { ArtifactInfo } from "../api";
import { artifactStatus, clearable, expiryOf, livesAt, promoteBlocked, promoteVersions, promotion } from "./shown";

// An artifact's lifetime, apart from drawing: when it expires, as the list
// says it, what Clear expired takes, and what the Promote dialog asks and
// sends.

const DAY = 24 * 60 * 60 * 1000;
const now = Date.parse("2026-11-16T12:00:00Z");
const at = (days: number) => new Date(now + days * DAY).toISOString();

const art = (id: string, a: Partial<ArtifactInfo> = {}): ArtifactInfo => ({
  id,
  title: id,
  status: "live",
  folder: `2026-10-16-${id}`,
  where: `.ws/artifacts/2026-10-16-${id}/`,
  made: "2026-10-16T12:00:00Z",
  versions: 1,
  ...a,
});

describe("expiry", () => {
  it("says when, as ws does, and warns in the last week", () => {
    expect(expiryOf(art("a1", { expires: at(20) }), now)).toEqual({ words: "expires in 20 days", soon: false, expired: false });
    expect(expiryOf(art("a1", { expires: at(4.5) }), now)).toEqual({ words: "expires in 4 days", soon: true, expired: false });
    expect(expiryOf(art("a1", { expires: at(1.2) }), now)?.words).toBe("expires tomorrow");
    expect(expiryOf(art("a1", { expires: at(0.2) }), now)?.words).toBe("expires today");
    expect(expiryOf(art("a1", { expires: at(-1) }), now)).toEqual({ words: "expired", soon: false, expired: true });
  });

  it("is nothing for one that doesn't expire", () => {
    expect(expiryOf(art("a1"), now)).toBeUndefined();
    expect(expiryOf(art("a1", { expires: "not a time" }), now)).toBeUndefined();
  });

  it("draws an expired one as expired, but in flight or gone first", () => {
    expect(artifactStatus(art("a1", { expired: true })).words).toBe("Expired");
    expect(artifactStatus(art("a1", { expired: true, gone: true })).words).toBe("Gone");
    expect(artifactStatus(art("a1", { status: "promoted" })).words).toBe("Promoted");
  });

  it("clears the live and expired", () => {
    const all = [
      art("a1", { expired: true }),
      art("a2", { expired: true }),
      art("a3", { expires: at(3) }),
      art("a4", { status: "promoted" }),
      art("a5", { status: "discarded", expired: true }),
      art("a6", { expired: true, gone: true }),
    ];
    expect(clearable(all).map((a) => a.id)).toEqual(["a1", "a2"]);
  });
});

describe("promoting", () => {
  const home = { where: "records:artifacts/glow/", repo: "records", version: 2, at: "2026-11-16T12:00:00Z" };

  it("chooses among its versions, newest first", () => {
    expect(promoteVersions([{ n: 1 }, { n: 3 }, { n: 2 }])).toEqual([3, 2, 1]);
  });

  it("asks where it's to live once: where it lives isn't asked again", () => {
    // The first time: what the workspace offers, or this machine.
    expect(livesAt({}, "records:artifacts/glow/")).toBe("offered");
    expect(livesAt({}, undefined)).toBe("here");
    // Promoted to stay here before, it opens on that, whatever's offered.
    expect(livesAt({ here: true }, "records:artifacts/glow/")).toBe("here");
    // It lives in a repo: that's where it goes.
    expect(livesAt({ home }, undefined)).toBe("home");
  });

  it("sends what it went into, or the mission to start, and where it's to live", () => {
    expect(promotion(2, "nothing", "glow")).toEqual({ version: 2 });
    expect(promotion(2, "mission", " glow ")).toEqual({ version: 2, into: "glow" });
    expect(promotion(1, "start", "Build the glow")).toEqual({ version: 1, start: "Build the glow" });
    // One that lives in a repo says nothing of where: the server knows.
    expect(promotion(5, "nothing", "", "home", "records:artifacts/glow/")).toEqual({ version: 5 });
    expect(promotion(1, "document", "records:plans/glow.md", "offered", "records:artifacts/glow/")).toEqual({ version: 1, into: "records:plans/glow.md", home: "records:artifacts/glow/" });
    expect(promotion(1, "nothing", "", "other", " records:kept/glow/ ")).toEqual({ version: 1, home: "records:kept/glow/" });
    expect(promotion(1, "nothing", "", "here", "records:artifacts/glow/")).toEqual({ version: 1, here: true });
  });

  it("says what's missing, and that a document cites only one that lives in a repo", () => {
    expect(promoteBlocked("nothing", "")).toBe("");
    expect(promoteBlocked("mission", "")).toBe("Choose the mission.");
    expect(promoteBlocked("start", " ")).toBe("Describe the mission.");
    expect(promoteBlocked("document", "records:plans/glow.md", "home")).toBe("");
    expect(promoteBlocked("document", "records:plans/glow.md", "offered", "records:artifacts/glow/")).toBe("");
    expect(promoteBlocked("document", "records:plans/glow.md", "here")).toContain("A document can only cite it from a repo: choose a folder in one.");
    expect(promoteBlocked("nothing", "", "other", "")).toBe("Name the folder, as repo:folder/.");
    expect(promoteBlocked("nothing", "", "other", "records:")).toBe("Name the folder, as repo:folder/.");
    expect(promoteBlocked("nothing", "", "other", "records:kept/glow/")).toBe("");
  });
});
