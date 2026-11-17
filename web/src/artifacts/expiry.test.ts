import { describe, expect, it } from "vitest";
import type { ArtifactInfo } from "../api";
import { artifactStatus, clearable, expiryOf, promoteBlocked, promoteVersions, promotion } from "./shown";

// An artifact's lifetime, apart from drawing: when it expires, as the list
// says it, what Clear expired takes, and what the Promote dialog sends.

const DAY = 24 * 60 * 60 * 1000;
const now = Date.parse("2026-11-16T12:00:00Z");
const at = (days: number) => new Date(now + days * DAY).toISOString();

const art = (id: string, a: Partial<ArtifactInfo> = {}): ArtifactInfo => ({
  id,
  title: id,
  status: "live",
  place: ".ws",
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

  it("clears the live and expired, here and in the repo", () => {
    const all = [
      art("a1", { expired: true, place: "records" }),
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
  it("chooses among the versions that have landed, newest first", () => {
    expect(promoteVersions([{ n: 1 }, { n: 3 }, { n: 2 }, { n: 4, capsule: "records/glow" }])).toEqual([3, 2, 1]);
  });

  it("sends what it went into, or the mission to start", () => {
    expect(promotion(2, "nothing", "glow", false)).toEqual({ version: 2 });
    expect(promotion(2, "mission", " glow ", false)).toEqual({ version: 2, into: "glow" });
    expect(promotion(1, "document", "records:plans/glow.md", true)).toEqual({ version: 1, into: "records:plans/glow.md", move: true });
    expect(promotion(1, "start", "Build the glow", false)).toEqual({ version: 1, start: "Build the glow" });
  });

  it("says what's missing, and that a document cites only one in a repo", () => {
    expect(promoteBlocked("nothing", "", true, false, "records")).toBe("");
    expect(promoteBlocked("mission", "", false, false)).toBe("Choose the mission.");
    expect(promoteBlocked("start", " ", false, false)).toBe("Say what the mission is.");
    expect(promoteBlocked("document", "records:plans/glow.md", false, false, "records")).toBe("");
    expect(promoteBlocked("document", "records:plans/glow.md", true, true, "records")).toBe("");
    expect(promoteBlocked("document", "records:plans/glow.md", true, false, "records")).toContain("move it to records as you promote it");
    expect(promoteBlocked("document", "records:plans/glow.md", true, false)).toContain("there's no repo for artifacts");
  });
});
