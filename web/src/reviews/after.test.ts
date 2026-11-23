import { expect, test } from "vitest";
import type { PR, PRState, ReviewRow } from "../api";
import { layersLeft, rowAfter } from "./after";

const pr = (number: number, state: PRState = "needs-review"): PR => ({ repo: "demo", number, title: `#${number}`, author: "ada", url: "", branch: "", base: "main", additions: 1, deletions: 0, files: 1, state, words: "" });
const row = (p: PR): ReviewRow => ({ key: `demo#${p.number}`, pr: p });
const stackRow = (...layers: PR[]): ReviewRow => ({
  key: `demo#stack-${layers[0].number}`,
  stack: { repo: "demo", title: "a stack", author: "ada", base: "main", layers, additions: 1, deletions: 0, state: "needs-review", words: "", open: layers[0].number },
});

test("a pull request on its own leaves nothing once it's reviewed", () => {
  expect(layersLeft([], [4])).toEqual([]);
});

test("a stack's layers that need you are left until each is posted", () => {
  const layers = [pr(1), pr(2, "back-with-you"), pr(3)];
  expect(layersLeft(layers, [1]).map((l) => l.number)).toEqual([2, 3]);
  expect(layersLeft(layers, [1, 2, 3])).toEqual([]);
});

test("a layer that asks nothing of you isn't left", () => {
  const layers = [pr(1), pr(2, "waiting-on-them"), pr(3, "approved"), pr(4, "drafts")];
  expect(layersLeft(layers, [1])).toEqual([]);
});

test("the row below the one reviewed is next", () => {
  const rows = [row(pr(1)), row(pr(2)), row(pr(3))];
  expect(rowAfter(rows, "demo", 2)).toBe("demo#3");
});

test("the last row's next is the one above", () => {
  const rows = [row(pr(1)), row(pr(2))];
  expect(rowAfter(rows, "demo", 2)).toBe("demo#1");
});

test("a layer is found in its stack's row", () => {
  const rows = [stackRow(pr(1), pr(2)), row(pr(3))];
  expect(rowAfter(rows, "demo", 2)).toBe("demo#3");
});

test("a pull request that isn't in the list, or is all of it, has no next", () => {
  expect(rowAfter([row(pr(1))], "demo", 9)).toBeUndefined();
  expect(rowAfter([row(pr(1))], "demo", 1)).toBeUndefined();
  expect(rowAfter([row(pr(1))], "other", 1)).toBeUndefined();
});
