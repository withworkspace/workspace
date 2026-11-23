import { describe, expect, it } from "vitest";
import { DOCK_STEP, keyed, sized } from "./dockSize";

describe("a dock's size", () => {
  it("grows as its edge is dragged away from it", () => {
    expect(sized("beside", 420, -40, 0, 300, 760)).toBe(460);
    expect(sized("beside", 420, 40, 0, 300, 760)).toBe(380);
    expect(sized("below", 290, 0, -30, 140, 620)).toBe(320);
    expect(sized("below", 290, 0, 30, 140, 620)).toBe(260);
  });

  it("takes the drag along its own axis only", () => {
    expect(sized("beside", 420, 0, 200, 300, 760)).toBe(420);
    expect(sized("below", 290, 200, 0, 140, 620)).toBe(290);
  });

  it("stays within its least and its most", () => {
    expect(sized("beside", 420, 500, 0, 300, 760)).toBe(300);
    expect(sized("beside", 420, -500, 0, 300, 760)).toBe(760);
  });

  it("moves by a step for the arrows of its side", () => {
    expect(keyed("beside", "ArrowLeft", 420, 300, 760)).toBe(420 + DOCK_STEP);
    expect(keyed("beside", "ArrowRight", 420, 300, 760)).toBe(420 - DOCK_STEP);
    expect(keyed("below", "ArrowUp", 290, 140, 620)).toBe(290 + DOCK_STEP);
    expect(keyed("below", "ArrowDown", 290, 140, 620)).toBe(290 - DOCK_STEP);
  });

  it("goes to its least and most on Home and End", () => {
    expect(keyed("beside", "Home", 420, 300, 760)).toBe(300);
    expect(keyed("below", "End", 290, 140, 620)).toBe(620);
  });

  it("leaves the other side's arrows, and other keys, alone", () => {
    expect(keyed("beside", "ArrowUp", 420, 300, 760)).toBeUndefined();
    expect(keyed("below", "ArrowLeft", 290, 140, 620)).toBeUndefined();
    expect(keyed("beside", "a", 420, 300, 760)).toBeUndefined();
  });
});
