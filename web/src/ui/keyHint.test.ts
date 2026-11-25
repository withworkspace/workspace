import { afterEach, describe, expect, it } from "vitest";
import { setPref } from "../prefs";
import { keyHint, pointed, titleWithKeys } from "./keyHint";
import { keysSaid } from "./Keys";
import { getToasts, setToasts } from "./toast";

afterEach(() => {
  setPref("keyHints", false);
  setToasts([]);
});

describe("a key hint", () => {
  it("isn't said unless you've asked for hints", () => {
    keyHint("v");
    expect(getToasts()).toEqual([]);
  });

  it("says the key, once you have", () => {
    setPref("keyHints", true);
    keyHint("o");
    expect(getToasts().map((t) => t.keys)).toEqual(["o"]);
  });

  it("isn't said over itself while it's still being said, but is for another key, and again once it's gone", () => {
    setPref("keyHints", true);
    keyHint("j");
    keyHint("j");
    keyHint("k");
    keyHint("j");
    expect(getToasts().map((t) => t.keys)).toEqual(["j", "k"]);
    setToasts([]);
    keyHint("j");
    expect(getToasts().map((t) => t.keys)).toEqual(["j"]);
  });

  it("is for a mouse or a pen, not the keys' own press, a screen reader's or a finger's", () => {
    expect(["mouse", "pen", "keyboard", "virtual", "touch"].filter((pointerType) => pointed({ pointerType }))).toEqual(["mouse", "pen"]);
  });
});

describe("a title with its key", () => {
  it("says the key after what it does, as it's read", () => {
    expect(titleWithKeys("Review its changes", "v")).toBe("Review its changes (v)");
    expect(titleWithKeys("Close", "Escape")).toBe("Close (esc)");
    expect(titleWithKeys("Open it", "Enter")).toBe("Open it (⏎)");
  });

  it("is either alone, where that's all there is", () => {
    expect(titleWithKeys("Review its changes", undefined)).toBe("Review its changes");
    expect(titleWithKeys(undefined, "v")).toBe("v");
    expect(titleWithKeys(undefined, undefined)).toBeUndefined();
  });
});

describe("keysSaid", () => {
  it("reads a key as a key map names it", () => {
    expect(keysSaid("Escape")).toEqual(["esc"]);
    expect(keysSaid("Enter")).toEqual(["⏎"]);
    expect(keysSaid("ArrowDown")).toEqual(["↓"]);
    expect(keysSaid("mod+Enter")[0]).toMatch(/^(⌘|Ctrl\+)⏎$/);
  });

  it("leaves a letter, and a key it has no sign for, as written", () => {
    expect(keysSaid("g m")).toEqual(["g", "m"]);
    expect(keysSaid("D")).toEqual(["D"]);
    expect(keysSaid("esc")).toEqual(["esc"]);
    expect(keysSaid("Tab")).toEqual(["Tab"]);
    expect(keysSaid("mod+K")[0]).toMatch(/^(⌘|Ctrl\+)K$/);
  });
});
