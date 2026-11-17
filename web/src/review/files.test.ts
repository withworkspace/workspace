import { expect, test } from "vitest";
import type { DiffFile } from "../api";
import { groupFiles } from "./files";

const file = (path: string, skim?: DiffFile["skim"]): DiffFile => ({ path, status: "modified", additions: 1, deletions: 0, hunks: [], skim });

test("a folder's files are a group, named without what they all share", () => {
  const g = groupFiles([
    file("src/portfolio/adaptation/engine.ts"),
    file("src/portfolio/adaptation/engine.test.ts"),
    file("src/portfolio/limits.ts"),
    file("src/sensitivity-matrix/solver.ts"),
    file("package-lock.json", "lockfile"),
    file("src/gen/api.pb.go", "generated"),
  ]);
  expect(g.common).toBe("src/");
  expect(g.groups.map((x) => [x.label, x.files])).toEqual([
    ["portfolio/adaptation/", [0, 1]],
    ["portfolio/", [2]],
    ["sensitivity-matrix/", [3]],
    ["To skim", [4, 5]],
  ]);
  expect(g.groups[0].title).toBe("src/portfolio/adaptation/");
  expect(g.groups[3].skim).toBe(true);
});

test("files in one folder share all of it, and files at the top share nothing", () => {
  const one = groupFiles([file("internal/web/a.go"), file("internal/web/b.go")]);
  expect(one.common).toBe("internal/web/");
  expect(one.groups.map((x) => x.label)).toEqual([""]);

  const top = groupFiles([file("README.md"), file("cmd/main.go")]);
  expect(top.common).toBe("");
  expect(top.groups.map((x) => [x.label, x.title])).toEqual([
    ["", "The top of the repo"],
    ["cmd/", "cmd/"],
  ]);

  expect(groupFiles([]).groups).toEqual([]);
  expect(groupFiles([file("go.sum", "lockfile")]).common).toBe("");
});

test("a guide's order: its groups, and what it doesn't place after them (or, stale, before)", async () => {
  const { groupByOrder, orderFiles } = await import("./files");
  const files = [file("a/x.go"), file("a/x_test.go"), file("b/y.go"), file("docs/README.md", "renamed"), file("go.sum", "lockfile"), file("new.go")];
  const order = {
    key: "k",
    groups: [
      { name: "The change", why: "Start here.", files: ["b/y.go", "a/x.go", "gone.go"] },
      { name: "Moves", skim: true, files: ["docs/README.md", "a/x_test.go", "b/y.go"] },
    ],
    notes: { "b/y.go": "the type" },
  };
  const ordered = orderFiles(files, order);
  expect(ordered.map((f) => [f.path, f.skim])).toEqual([
    ["b/y.go", undefined],
    ["a/x.go", undefined],
    ["docs/README.md", "renamed"],
    ["a/x_test.go", "guide"],
    ["go.sum", "lockfile"],
    ["new.go", undefined],
  ]);
  // Each says why it's read there, for its heading in the diff.
  expect(groupByOrder(ordered, order).groups.map((g) => [g.label, g.files, !!g.skim, g.why])).toEqual([
    ["The change", [0, 1], false, "Start here."],
    ["Moves", [2, 3], true, undefined],
    ["Not placed", [4, 5], false, "The guide didn't say where these are read"],
  ]);
  // The change has moved on: what the guide doesn't place may be new.
  const stale = { ...order, stale: true };
  const again = orderFiles(files, stale);
  expect(again.slice(0, 2).map((f) => f.path)).toEqual(["go.sum", "new.go"]);
  expect(groupByOrder(again, stale).groups.map((g) => g.label)).toEqual(["Not in the guide", "The change", "Moves"]);
});
