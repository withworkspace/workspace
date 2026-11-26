import { describe, expect, it } from "vitest";
import type { DiffFile, ReviewNote } from "../api";
import { commentEntries, inFilter, stepFrom } from "./comments";
import type { Row } from "./model";

const note = (o: Partial<ReviewNote>): ReviewNote => ({ id: "c1", path: "a.go", side: "new", line: 3, endLine: 3, commit: "abc", excerpt: [], kind: "nit", body: "Say why.", status: "open", thread: [], ...o });
const comment = (file: number, n: ReviewNote, draft = false): Row => ({ type: "comment", key: `${file}:c:${n.id}`, file, placed: { note: n, review: "r1", draft } });
const files = [{ path: "a.go" }, { path: "b.go" }] as DiffFile[];

describe("commentEntries", () => {
  const rows: Row[][] = [
    [{ type: "file", key: "0", file: 0 }, comment(0, note({ id: "c1" }), true), comment(0, note({ id: "c2", line: 7, endLine: 9, side: "old", status: "resolved", thread: [{ by: "jun", at: "", body: "Done." }] }))],
    [comment(1, note({ id: "c3", path: "b.go", status: "addressed" }))],
  ];
  const orphans: Row[] = [comment(2, note({ id: "c4", path: "c.go", status: "wontfix" }))];
  const list = commentEntries(rows, orphans, files);

  it("is every comment in reading order, files' then those on files the diff hasn't", () => {
    expect(list.map((c) => c.id)).toEqual(["c1", "c2", "c3", "c4"]);
    expect(list.map((c) => c.path)).toEqual(["a.go", "a.go", "b.go", "c.go"]);
  });

  it("says where it is, how it stands and what was said last", () => {
    expect(list[0]).toMatchObject({ key: "0:c:c1", lines: "L3", state: "draft", removed: false });
    expect(list[1]).toMatchObject({ lines: "L7–9", removed: true, state: "resolved", last: { by: "jun", body: "Done." }, replies: 1 });
  });

  it("files a draft under drafts, addressed under open, won't fix under resolved", () => {
    const ids = (f: Parameters<typeof inFilter>[0]) => list.filter(inFilter(f)).map((c) => c.id);
    expect(ids("draft")).toEqual(["c1"]);
    expect(ids("open")).toEqual(["c3"]);
    expect(ids("resolved")).toEqual(["c2", "c4"]);
    expect(ids("all")).toHaveLength(4);
  });
});

describe("stepFrom", () => {
  const list = commentEntries([[comment(0, note({ id: "c1" })), comment(0, note({ id: "c2" }))]], [], files);
  it("goes on to the next, and round from the last to the first", () => {
    expect(stepFrom(list, "0:c:c1", 1)?.id).toBe("c2");
    expect(stepFrom(list, "0:c:c2", 1)?.id).toBe("c1");
    expect(stepFrom(list, "0:c:c1", -1)?.id).toBe("c2");
  });
  it("starts at the first (or, going back, the last) when none's been gone to", () => {
    expect(stepFrom(list, undefined, 1)?.id).toBe("c1");
    expect(stepFrom(list, undefined, -1)?.id).toBe("c2");
    expect(stepFrom([], undefined, 1)).toBeUndefined();
  });
});
