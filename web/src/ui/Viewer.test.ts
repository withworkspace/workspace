import { describe, expect, it, vi } from "vitest";
import { unview, view } from "./Viewer";

describe("what the viewer shows", () => {
  const a = { kind: "image" as const, src: "https://example.com/a.png" };
  const b = { kind: "image" as const, src: "https://example.com/b.png" };

  it("says it's closed once it isn't shown: closed, or another shown in its place", () => {
    const closedA = vi.fn();
    const closedB = vi.fn();
    view(a, closedA);
    expect(closedA).not.toHaveBeenCalled();
    view(b, closedB);
    expect(closedA).toHaveBeenCalledTimes(1);
    view(null);
    expect(closedB).toHaveBeenCalledTimes(1);
    view(null);
    expect(closedB).toHaveBeenCalledTimes(1);
  });

  it("unview takes down only what's still shown", () => {
    const closedA = vi.fn();
    const closedB = vi.fn();
    view(a, closedA);
    view(b, closedB);
    unview(a);
    expect(closedB).not.toHaveBeenCalled();
    unview(b);
    expect(closedB).toHaveBeenCalledTimes(1);
  });
});
