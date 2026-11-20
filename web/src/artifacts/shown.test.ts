import { describe, expect, it } from "vitest";
import type { ArtifactFound, ArtifactInfo } from "../api";
import { ANYWHERE, artifactStatus, behind, BY_HAND, filesWords, type FoundFolder, foundTree, keptIcon, keptTitle, keptWords, fromRef, listed, missionTalk, sizeWords, sources, stepVersion, versionOf } from "./shown";

const art = (id: string, status: ArtifactInfo["status"], from: string[] = []): ArtifactInfo => ({
  id,
  title: id,
  status,
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
    expect(artifactStatus({ status: "live", expired: true }).words).toBe("Expired");
  });
});

describe("where it's kept", () => {
  const home = { where: "records:artifacts/x/", repo: "records", version: 2, at: "2026-11-12T18:46:00Z" };
  const v = (n: number) => ({ n, at: "", by: "you" });

  it("is this machine, and the repo it lives in, promoted there", () => {
    expect(keptWords({})).toBe("This machine");
    expect(keptIcon({})).toBe("monitor");
    expect(keptWords({ home })).toBe("records · v2");
    expect(keptIcon({ home })).toBe("branch");
    expect(keptTitle({ where: ".ws/artifacts/x/" })).toBe("On this machine only: .ws/artifacts/x/");
    expect(keptTitle({ where: ".ws/artifacts/x/", home })).toBe("v2 lives in records:artifacts/x/, for everyone with records. Every version is on this machine: .ws/artifacts/x/");
  });

  it("says how many versions are newer than the one that's there", () => {
    expect(behind({ latest: v(5) })).toBe(0);
    expect(behind({ home, latest: v(2) })).toBe(0);
    expect(behind({ home, latest: v(5) })).toBe(3);
    // One promoted back to an earlier version has nothing newer to put there.
    expect(behind({ home: { ...home, version: 4 }, latest: v(4) })).toBe(0);
  });
});
describe("how much a version is", () => {
  it("says its size alone for one that's its page only, and how many files for one with more", () => {
    expect(filesWords(undefined, 2048)).toBe("2 KB");
    expect(filesWords(1, 2048)).toBe("2 KB");
    expect(filesWords(9, 5 * 1024)).toBe("9 files, 5 KB");
    expect(filesWords(3, 3 << 20)).toBe("3 files, 3 MB");
  });
});

describe("the artifacts ws found in the repos, as they're organised there", () => {
  const found = (title: string, more: Partial<ArtifactFound> = {}): ArtifactFound => ({ name: `records/artifacts/${title}/`, where: `records:artifacts/${title}/`, repo: "records", label: "Records", title, version: "", frame: "", changed: "2026-11-20T10:00:00Z", ...more });
  // said is a tree in a line: each folder with how many are in it, its own artifacts, and its folders.
  const said = (folders: FoundFolder[]): string => folders.map((f) => `${f.name}(${f.count})[${f.found.map((a) => a.title).join(" ")}]${f.folders.length ? `{${said(f.folders)}}` : ""}`).join(" ");

  it("are a tree for each repo, as ws calls it, of the folders they're grouped in", () => {
    const tree = foundTree([found("q4", { group: "2026/q4" }), found("flat"), found("guide", { repo: "docs", label: "Docs", group: "boards" }), found("board", { group: "boards" })]);
    // A folder with only a folder in it (2026) is there, and counts what's under it.
    expect(said(tree)).toBe("Docs(1)[]{boards(1)[guide]} Records(3)[flat]{2026(1)[]{q4(1)[q4]} boards(1)[board]}");
    expect(tree[1]?.folders[0]?.folders[0]).toMatchObject({ name: "q4", repo: "records", path: "2026/q4" });
  });

  it("none are no tree, and one repo with nothing grouped is a tree of one", () => {
    expect(foundTree([])).toEqual([]);
    expect(said(foundTree([found("a"), found("b")]))).toBe("Records(2)[a b]");
  });
});
