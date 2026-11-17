import { describe, expect, it } from "vitest";
import type { ArtifactInfo } from "../api";
import { ANYWHERE, artifactStatus, BY_HAND, keptTitle, keptWords, fromRef, listed, missionTalk, sizeWords, sources, stepVersion, versionOf } from "./shown";

const art = (id: string, status: ArtifactInfo["status"], from: string[] = []): ArtifactInfo => ({
  id,
  title: id,
  status,
  place: ".ws",
  folder: `2026-11-12-${id}`,
  where: `.ws/artifacts/2026-11-12-${id}/`,
  made: "2026-11-12T18:46:00Z",
  versions: 1,
  from: from.map((f) => ({ from: f, title: `The ${f}` })),
});

describe("the list", () => {
  const all = [art("a3", "live", ["w9", "glow"]), art("a2", "promoted", ["glow"]), art("a1", "discarded"), art("a4", "live")];
  const ids = (list: ArtifactInfo[]) => list.map((a) => a.id);

  it("leaves the discarded off, unless they're asked for", () => {
    expect(ids(listed(all, "all", ANYWHERE))).toEqual(["a3", "a2", "a4"]);
    expect(ids(listed(all, "live", ANYWHERE))).toEqual(["a3", "a4"]);
    expect(ids(listed(all, "promoted", ANYWHERE))).toEqual(["a2"]);
    expect(ids(listed(all, "discarded", ANYWHERE))).toEqual(["a1"]);
  });

  it("filters by where versions came from", () => {
    expect(ids(listed(all, "all", "glow"))).toEqual(["a3", "a2"]);
    expect(ids(listed(all, "all", "w9"))).toEqual(["a3"]);
    expect(ids(listed(all, "all", BY_HAND))).toEqual(["a4"]);
    expect(ids(listed(all, "discarded", BY_HAND))).toEqual(["a1"]);
    expect(sources(all)).toEqual([
      { from: "w9", title: "The w9" },
      { from: "glow", title: "The glow" },
    ]);
  });
});

describe("a version", () => {
  it("came from a conversation or a mission", () => {
    expect(fromRef("w9")).toEqual({ kind: "talk", id: "w9" });
    expect(fromRef("horizon-glow")).toEqual({ kind: "mission", slug: "horizon-glow" });
    // Confirmed in a mission's conversation: drawn as its mission.
    expect(fromRef("horizon-glow/t3")).toEqual({ kind: "mission", slug: "horizon-glow" });
    expect(missionTalk("horizon-glow/t3")).toEqual({ slug: "horizon-glow", id: "t3" });
    expect(missionTalk("horizon-glow")).toBeUndefined();
    expect(missionTalk("w9")).toBeUndefined();
  });

  it("steps to the one before or after, and stops at the ends", () => {
    const vs = [{ n: 1 }, { n: 2 }, { n: 4 }];
    expect(stepVersion(vs, 2, 1)).toBe(4);
    expect(stepVersion(vs, 2, -1)).toBe(1);
    expect(stepVersion(vs, 4, 1)).toBe(4);
    expect(stepVersion(vs, 1, -1)).toBe(1);
    expect(stepVersion(vs, 9, -1)).toBe(4);
  });

  it("is read from the address", () => {
    expect(versionOf("v2")).toBe(2);
    expect(versionOf("2")).toBe(2);
    expect(versionOf(undefined)).toBe(0);
    expect(versionOf("v0")).toBe(0);
    expect(versionOf("two")).toBe(0);
  });

  it("says its size, and where it stands", () => {
    expect(sizeWords(900)).toBe("900 bytes");
    expect(sizeWords(12_800)).toBe("13 KB");
    expect(sizeWords(1 << 20)).toBe("1 MB");
    expect(artifactStatus({ status: "live" }).words).toBe("Live");
    expect(artifactStatus({ status: "live", gone: true }).words).toBe("Gone");
    expect(artifactStatus({ status: "promoted" }).tone).toBe("done");
    expect(artifactStatus({ status: "live", inFlight: "records/glow" }).words).toBe("In flight");
    expect(artifactStatus({ status: "live", latest: { n: 3, at: "", by: "c1", capsule: "records/glow" } }).words).toBe("In flight");
    expect(artifactStatus({ status: "promoted", inFlight: "records/glow" }).words).toBe("Promoted");
  });
});

describe("where it's kept", () => {
  it("is this machine, or the repo, and in flight in a capsule", () => {
    expect(keptWords(".ws")).toBe("This machine");
    expect(keptWords("records")).toBe("records");
    expect(keptTitle({ place: ".ws", where: ".ws/artifacts/x/" })).toBe("On this machine only: .ws/artifacts/x/");
    expect(keptTitle({ place: "records", where: "records:artifacts/x/" })).toBe("In records, for everyone with it: records:artifacts/x/");
    expect(keptTitle({ place: "records", where: "records/glow:artifacts/x/", inFlight: "records/glow" })).toBe("In records/glow, not landed: records/glow:artifacts/x/");
  });
});
