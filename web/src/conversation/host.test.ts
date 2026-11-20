import { beforeEach, describe, expect, test } from "vitest";
import { createHost, fit, givenWith, preferred, type Status } from "./host";

// Node has no localStorage: a map stands in.
const kept = new Map<string, string>();
Object.assign(globalThis, {
  localStorage: { getItem: (k: string) => kept.get(k) ?? null, setItem: (k: string, v: string) => void kept.set(k, v), clear: () => kept.clear() },
});
beforeEach(() => kept.clear());

describe("fit", () => {
  test("the form wanted, where it's allowed and there's room", () => {
    expect(fit("docked", ["card", "docked", "popover"], false)).toBe("docked");
    expect(fit("popover", ["card", "popover"], false)).toBe("popover");
  });
  test("docked floats where it's narrow", () => {
    expect(fit("docked", ["card", "docked", "popover"], true)).toBe("popover");
  });
  test("one not allowed is the first that is and fits, or the card", () => {
    expect(fit("docked", ["card", "popover"], false)).toBe("card");
    expect(fit("card", ["docked", "popover"], true)).toBe("popover");
    expect(fit("popover", ["docked"], true)).toBe("card");
    expect(fit("docked", [], false)).toBe("card");
  });
});

describe("the host", () => {
  test("set is taken only when it changes", () => {
    const h = createHost("sketch");
    let heard = 0;
    h.subscribe(() => heard++);
    h.set({ ask: true, allow: ["card"] });
    h.set({ ask: true, allow: ["card"] });
    expect(heard).toBe(1);
    expect(h.state.settings).toEqual({ ask: true, allow: ["card"] });
  });

  test("the person's move is remembered for the owner, and opens with it next time", () => {
    const h = createHost("sketch");
    h.move("docked");
    expect(h.state).toMatchObject({ open: true, want: "docked" });
    expect(preferred("sketch")).toBe("docked");
    expect(preferred("jira")).toBeUndefined();
    const again = createHost("sketch");
    again.open({ message: "hi" });
    expect(again.state).toMatchObject({ open: true, want: "docked", opening: { n: 1, message: "hi" } });
    again.move(undefined);
    expect(again.state.open).toBe(false);
  });

  test("a page's open with as asks for that form, and isn't remembered", () => {
    const h = createHost("sketch");
    h.open({ as: "popover" });
    expect(h.state.want).toBe("popover");
    expect(preferred("sketch")).toBeUndefined();
    h.took(1);
    expect(h.state.opening).toBeUndefined();
  });

  test("the page hears where it is only when that changes", () => {
    const h = createHost("sketch");
    const seen: Status[] = [];
    h.onChange((s) => seen.push(s));
    h.shown({ open: false, working: false });
    h.shown({ open: true, as: "card", working: true });
    h.shown({ open: true, as: "card", working: true });
    h.shown({ open: true, as: "card", working: false });
    expect(seen).toEqual([
      { open: true, as: "card", working: true },
      { open: true, as: "card", working: false },
    ]);
  });

  test("forget drops what the page said, and closes", () => {
    const h = createHost("sketch");
    h.set({ ask: true, allow: ["card"], context: "x" });
    h.open();
    h.forget();
    expect(h.state).toMatchObject({ settings: { ask: false, allow: [] }, open: false });
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
