import { afterEach, describe, expect, it } from "vitest";
import { setPref } from "../prefs";
import { clicked, hintOnClick, keyHint, keyProps, pointed, titleWithKeys } from "./keyHint";
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

describe("a click on what isn't one of the library's parts", () => {
  const mouse = { detail: 1, nativeEvent: { pointerType: "mouse" } };

  it("is the pointer's unless it was the keys' own, a screen reader's or a finger's", () => {
    expect(clicked(mouse)).toBe(true);
    expect(clicked({ detail: 1 })).toBe(true); // a browser that doesn't say which pointer
    expect(clicked({ detail: 0, nativeEvent: { pointerType: "" } })).toBe(false); // ⏎ on a link
    expect(clicked({ detail: 1, nativeEvent: { pointerType: "touch" } })).toBe(false);
  });

  it("hints at the key, then does what the click does", () => {
    setPref("keyHints", true);
    let did = 0;
    const props = keyProps("o", "Open it in Files", () => did++);
    expect(props.title).toBe("Open it in Files (o)");
    props.onClick?.(mouse);
    expect(did).toBe(1);
    expect(getToasts().map((t) => t.keys)).toEqual(["o"]);
    props.onClick?.({ detail: 0 });
    expect(did).toBe(2);
    expect(getToasts()).toHaveLength(1);
  });

  it("is the title and the handler as given, with no key", () => {
    const go = () => undefined;
    expect(keyProps(undefined, "Open it", go)).toEqual({ title: "Open it", onClick: go });
    expect(hintOnClick(undefined)).toBeUndefined();
  });
});

describe("a title with its key", () => {
  it("says the key after what it does, as it's read", () => {
    expect(titleWithKeys("Review its changes", "v")).toBe("Review its changes (v)");
    expect(titleWithKeys("Close", "Escape")).toBe("Close (esc)");
    expect(titleWithKeys("Open it", "Enter")).toBe("Open it (⏎)");
  });

  it("is the title alone with no key, and nothing with no title: a key by itself isn't a tooltip", () => {
    expect(titleWithKeys("Review its changes", undefined)).toBe("Review its changes");
    expect(titleWithKeys(undefined, "v")).toBeUndefined();
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
