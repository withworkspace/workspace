import { describe, expect, test } from "vitest";
import { createHost, givenWith, type Status } from "./host";

describe("the host", () => {
  test("set is taken only when it changes", () => {
    const h = createHost();
    let heard = 0;
    h.subscribe(() => heard++);
    h.set({ ask: true, allow: ["card"] });
    h.set({ ask: true, allow: ["card"] });
    expect(heard).toBe(1);
    expect(h.state.settings).toEqual({ ask: true, allow: ["card"] });
  });

  test("each asking to open is one of its own, until the view takes it", () => {
    const h = createHost();
    h.open({ message: "hi" });
    expect(h.state.opening).toEqual({ n: 1, message: "hi" });
    h.took(1);
    expect(h.state.opening).toBeUndefined();
    h.open({ message: "hi", as: "popover" });
    expect(h.state.opening).toEqual({ n: 2, message: "hi", as: "popover" });
    // One taken late isn't the one that's asking now.
    h.took(1);
    expect(h.state.opening?.n).toBe(2);
  });

  test("close is heard each time, and drops an opening nobody took", () => {
    const h = createHost();
    h.open();
    h.close();
    expect(h.state).toMatchObject({ closing: 1 });
    expect(h.state.opening).toBeUndefined();
    h.close();
    expect(h.state.closing).toBe(2);
  });

  test("the page hears where it is only when that changes", () => {
    const h = createHost();
    const seen: Status[] = [];
    h.onChange((s) => seen.push(s));
    h.shown({ open: false, working: false });
    h.shown({ open: true, as: "docked", working: true });
    h.shown({ open: true, as: "docked", working: true });
    h.shown({ open: true, as: "docked", working: false });
    expect(seen).toEqual([
      { open: true, as: "docked", working: true },
      { open: true, as: "docked", working: false },
    ]);
  });

  test("forget drops what the page said, and closes", () => {
    const h = createHost();
    h.set({ ask: true, allow: ["card"], context: "x" });
    h.open();
    h.forget();
    expect(h.state).toMatchObject({ settings: { ask: false, allow: [] }, closing: 1 });
    expect(h.state.opening).toBeUndefined();
  });
});

test("givenWith gives what there is, and nothing when there's nothing", () => {
  expect(givenWith("Sketch", { ask: true, allow: [] })).toBeUndefined();
  expect(givenWith("Sketch", { ask: true, allow: [], context: "Brief → Plan", holding: { label: "1 selected", text: "`plan`" } }, "more")).toEqual({
    from: "Sketch",
    page: "Brief → Plan",
    holding: "1 selected",
    holdingText: "`plan`",
    context: "more",
  });
});
