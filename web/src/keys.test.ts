import { createElement as h } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { claimCodeSearch, codeSearchKey, isCodeSearchKey, keysOf } from "./keys";
import { type Command, scope } from "./keymap/commands";
import { useCommand } from "./keymap/useCommands";

const key = (k: Partial<KeyboardEvent>) => ({ metaKey: false, ctrlKey: false, shiftKey: false, altKey: false, key: "", ...k });

describe("mod+shift+F", () => {
  it("is the key, however its letter's spelled", () => {
    expect(isCodeSearchKey(key({ metaKey: true, shiftKey: true, key: "F" }))).toBe(true);
    expect(isCodeSearchKey(key({ metaKey: true, shiftKey: true, key: "f" }))).toBe(true);
    expect(isCodeSearchKey(key({ ctrlKey: true, shiftKey: true, key: "F" }))).toBe(true);
    // The browser's find, and others, aren't.
    expect(isCodeSearchKey(key({ metaKey: true, key: "f" }))).toBe(false);
    expect(isCodeSearchKey(key({ shiftKey: true, key: "F" }))).toBe(false);
    expect(isCodeSearchKey(key({ metaKey: true, shiftKey: true, altKey: true, key: "f" }))).toBe(false);
  });

  it("opens Find's code search, where the page has no box of its own", () => {
    expect(codeSearchKey(false).to).toBe("find");
  });

  it("does nothing while an overlay is open", () => {
    expect(codeSearchKey(true).to).toBe("nothing");
    const release = claimCodeSearch(() => undefined);
    expect(codeSearchKey(true).to).toBe("nothing");
    release();
  });

  it("opens the page's own search box (the explorer's) where it has one", () => {
    let opened = 0;
    const release = claimCodeSearch(() => opened++);
    const does = codeSearchKey(false);
    expect(does.to).toBe("own");
    if (does.to === "own") does.open();
    expect(opened).toBe(1);
    release();
    expect(codeSearchKey(false).to).toBe("find");
  });

  it("goes to the box claimed last, and back to the one before when it goes", () => {
    const got: string[] = [];
    const outer = claimCodeSearch(() => got.push("outer"));
    const inner = claimCodeSearch(() => got.push("inner"));
    const a = codeSearchKey(false);
    if (a.to === "own") a.open();
    inner();
    const b = codeSearchKey(false);
    if (b.to === "own") b.open();
    outer();
    expect(got).toEqual(["inner", "outer"]);
  });
});

const tried = scope("test-keys", "Tried", {
  view: { title: "View", keys: "v" },
  zoom: { title: "Zoom in", keys: ["+", "="] },
});

describe("useCommand", () => {
  // What it gives back, as a button given it would read it.
  function Says({ c, enabled, did }: { c: Command; enabled?: boolean; did?: () => void }) {
    const fn = useCommand(c, did ?? (() => undefined), { enabled });
    fn();
    return h("i", null, keysOf(fn) ?? "none");
  }
  const said = (props: Parameters<typeof Says>[0]) => renderToStaticMarkup(h(Says, props));

  it("gives back what it was given, saying its command's key", () => {
    let did = 0;
    expect(said({ c: tried.view, did: () => did++ })).toBe("<i>v</i>");
    expect(did).toBe(1);
  });

  it("says the first of several keys", () => {
    expect(said({ c: tried.zoom })).toBe("<i>+</i>");
  });

  it("says no key while it isn't bound, and still does what it does", () => {
    let did = 0;
    expect(said({ c: tried.view, enabled: false, did: () => did++ })).toBe("<i>none</i>");
    expect(did).toBe(1);
  });

  it("is lost by a function around it", () => {
    const fn = Object.assign(() => undefined, { keys: "v" });
    expect(keysOf(fn)).toBe("v");
    expect(keysOf(() => fn())).toBeUndefined();
    expect(keysOf(undefined)).toBeUndefined();
  });
});
