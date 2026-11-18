import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { bursts } from "./load";

describe("bursts", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("tells of a change at once", () => {
    const tell = vi.fn();
    bursts(tell).changed();
    expect(tell).toHaveBeenCalledTimes(1);
    vi.advanceTimersByTime(1000);
    expect(tell).toHaveBeenCalledTimes(1);
  });

  it("tells once more of the changes that follow within a moment", () => {
    const tell = vi.fn();
    const b = bursts(tell, 80);
    for (let i = 0; i < 8; i++) {
      b.changed();
      vi.advanceTimersByTime(3);
    }
    expect(tell).toHaveBeenCalledTimes(1);
    vi.advanceTimersByTime(80);
    expect(tell).toHaveBeenCalledTimes(2);
    vi.advanceTimersByTime(1000);
    expect(tell).toHaveBeenCalledTimes(2);
  });

  it("tells at once again after the moment has passed", () => {
    const tell = vi.fn();
    const b = bursts(tell, 80);
    b.changed();
    vi.advanceTimersByTime(81);
    b.changed();
    expect(tell).toHaveBeenCalledTimes(2);
  });

  it("keeps up with changes that never stop, a moment apart", () => {
    const tell = vi.fn();
    const b = bursts(tell, 80);
    for (let i = 0; i < 100; i++) {
      b.changed();
      vi.advanceTimersByTime(10);
    }
    // A second of them: one at once, then one each 80ms.
    expect(tell.mock.calls.length).toBeGreaterThanOrEqual(12);
    expect(tell.mock.calls.length).toBeLessThanOrEqual(14);
  });

  it("tells of nothing once stopped", () => {
    const tell = vi.fn();
    const b = bursts(tell, 80);
    b.changed();
    b.changed();
    b.stop();
    vi.advanceTimersByTime(1000);
    expect(tell).toHaveBeenCalledTimes(1);
  });
});
