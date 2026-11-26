import { describe, expect, it } from "vitest";
import type { WhatsNewEntry, WhatsNewInfo } from "../../api";
import { ledeOf, releaseOf, releasesIn } from "./lede";

const entry = (slug: string, version?: string): WhatsNewEntry => ({ slug, version, title: slug, body: "" });
const info = (over: Partial<WhatsNewInfo>): WhatsNewInfo => ({ version: "v0.17.0", release: true, missing: false, unseen: [], entries: [], ...over });

describe("what What's new says of what it shows", () => {
  it("counts the releases, with no release as one", () => {
    expect(releasesIn([entry("a", "v0.16.0"), entry("b", "v0.16.0")])).toBe(1);
    expect(releasesIn([entry("a", "v0.16.0"), entry("b"), entry("c", "v0.15.0")])).toBe(3);
  });

  it("says since which release, for what's unseen", () => {
    const unseen = [entry("a", "v0.17.0"), entry("b", "v0.16.0")];
    expect(ledeOf(unseen, info({ seen: "v0.15.0", unseen }))).toBe("Since v0.15.0 · 2 things, across 2 releases");
  });

  it("says only how many, for everything", () => {
    const entries = [entry("a", "v0.17.0")];
    expect(ledeOf(entries, info({ seen: "v0.15.0", unseen: [entry("x")], entries }))).toBe("one thing");
    expect(ledeOf([], info({}))).toBe("");
  });

  it("says a build of a clone is one", () => {
    expect(ledeOf([entry("a")], info({ release: false }))).toMatch(/^Development build/);
  });

  it("says an entry's release, or that it's in none yet", () => {
    expect(releaseOf(entry("a", "v0.16.0"))).toBe("v0.16.0");
    expect(releaseOf(entry("a"))).toBe("Not released yet");
  });
});
