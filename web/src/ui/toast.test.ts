import { afterEach, describe, expect, it, vi } from "vitest";
import { leaveMs, saying, toast, toasts } from "./toast";

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

describe("a toast that's closed", () => {
  const motion = (reduce: boolean) => vi.stubGlobal("window", { matchMedia: () => ({ matches: reduce }) });

  it("stays a moment, leaving, so it's seen to go: then it's gone", () => {
    vi.useFakeTimers();
    motion(false);
    toast("Copied");
    const [t] = toasts.visibleToasts;
    toasts.close(t.key);
    expect(toasts.leaving(t.key)).toBe(true);
    expect(toasts.visibleToasts).toHaveLength(1);
    expect(saying()).toEqual([]);
    toasts.close(t.key); // closed again while it goes: once is enough
    vi.advanceTimersByTime(leaveMs);
    expect(toasts.visibleToasts).toEqual([]);
    expect(toasts.leaving(t.key)).toBe(false);
    vi.useRealTimers();
  });

  it("goes at once where motion's kept down", () => {
    motion(true);
    toast("Copied");
    toasts.close(toasts.visibleToasts[0].key);
    expect(toasts.visibleToasts).toEqual([]);
  });

  it("sent away by a fourth, leaves while the new one comes", () => {
    vi.useFakeTimers();
    motion(false);
    for (const n of [1, 2, 3, 4]) toast(`Said ${n}`);
    expect(saying().map((t) => t.text)).toEqual(["Said 4", "Said 3", "Said 2"]);
    expect(toasts.visibleToasts.map((t) => [t.content.text, toasts.leaving(t.key)])).toContainEqual(["Said 1", true]);
    vi.advanceTimersByTime(leaveMs);
    expect(toasts.visibleToasts).toHaveLength(3);
    vi.useRealTimers();
  });
});
