import { afterEach, describe, expect, it } from "vitest";
import { saying, toast, toasts } from "./toast";

afterEach(() => toasts.clear());

describe("toast", () => {
  it("says it, newest first, in the tone it's given (info, unsaid)", () => {
    toast("Copied the link");
    toast("Approved b3", { tone: "done" });
    expect(saying().map((t) => [t.text, t.tone])).toEqual([
      ["Approved b3", "done"],
      ["Copied the link", "info"],
    ]);
  });

  it("says three at most: a new one takes the oldest's place, and isn't kept waiting", () => {
    for (const n of [1, 2, 3, 4]) toast(`Said ${n}`);
    expect(saying().map((t) => t.text)).toEqual(["Said 4", "Said 3", "Said 2"]);
    expect(toasts.visibleToasts).toHaveLength(3);
  });

  it("keeps one with something to press longer, unless it's told how long", () => {
    toast("Parked", { action: { label: "Undo", run: () => undefined } });
    toast("Copied");
    toast("Read", { ms: 1200 });
    expect(toasts.visibleToasts.map((t) => t.timeout)).toEqual([1200, 3500, 6000]);
  });
});
