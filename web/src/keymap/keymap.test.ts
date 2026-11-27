import { afterEach, describe, expect, it } from "vitest";
import { keyName } from "../keys";
import { setPref } from "../prefs";
import { everywhereKeys } from "../shell/keys";
import { scope, scopeOf } from "./commands";
import { bind, forgetAll, holdKeys, pass, press } from "./dispatch";
import { frameKeys } from "./frame";
import { rowsOf } from "./help";
import { clashesOf, isOwn, keyOf, keysOf, resetKeys, setKeys, takeSuggested } from "./keymap";

// A press, as the dispatcher's given one.
function key(k: Partial<KeyboardEvent>): KeyboardEvent {
  let prevented = false;
  return {
    metaKey: false,
    ctrlKey: false,
    shiftKey: false,
    altKey: false,
    isComposing: false,
    key: "",
    code: "",
    target: null,
    ...k,
    get defaultPrevented() {
      return prevented;
    },
    preventDefault() {
      prevented = true;
    },
    stopPropagation() {},
  } as unknown as KeyboardEvent;
}
// pressed is a key pressed, as the window hears it: the capture, then
// after the focus has had it.
function pressed(k: Partial<KeyboardEvent>) {
  const e = key(k);
  press(e, true);
  press(e, false);
  return e;
}

const list = scope("test-list", "List", {
  next: { title: "Next", keys: ["j", "ArrowDown"], row: "Next / previous", moves: true },
  prev: { title: "Previous", keys: ["k", "ArrowUp"], row: "Next / previous", moves: true },
  first: { title: "First", keys: "g" },
  open: { title: "Open", keys: "Enter" },
  none: { title: "Has none", keys: [] },
  typed: { title: "Typed", keys: "```", fixed: true },
});
const go = scope("test-go", "Go", {
  missions: { title: "Go to Missions", keys: "g m" },
  send: { title: "Send", keys: "mod+Enter", fields: true },
});

afterEach(() => {
  forgetAll();
  resetKeys();
});

function did() {
  const got: string[] = [];
  const h = (n: string) => () => void got.push(n);
  return { got, h };
}

describe("keyName", () => {
  const k = (e: Partial<KeyboardEvent>) => keyName({ metaKey: false, ctrlKey: false, shiftKey: false, altKey: false, key: "", code: "", ...e });
  it("names a letter with ⌘ small, and says shift, however the browser spells it", () => {
    expect(k({ metaKey: true, shiftKey: true, key: "F" })).toBe("mod+shift+f");
    expect(k({ metaKey: true, shiftKey: true, key: "f" })).toBe("mod+shift+f");
    expect(k({ ctrlKey: true, key: "K" })).toBe("mod+k");
  });
  it("names a bare letter as it's typed, and shift only where the name doesn't say it", () => {
    expect(k({ key: "J", shiftKey: true })).toBe("J");
    expect(k({ key: "?", shiftKey: true })).toBe("?");
    expect(k({ key: "ArrowDown", shiftKey: true })).toBe("shift+ArrowDown");
    expect(k({ key: " " })).toBe("Space");
  });
  it("names a key with ⌥ by where it is, not what ⌥ makes it", () => {
    expect(k({ altKey: true, key: "å", code: "KeyA" })).toBe("alt+a");
    expect(k({ altKey: true, metaKey: true, key: "¡", code: "Digit1" })).toBe("mod+alt+1");
  });
});

describe("the dispatcher", () => {
  it("does a command by any of its keys, and says it took the key", () => {
    const { got, h } = did();
    bind({ scope: scopeOf(list), handlers: () => ({ next: h("next") }), modal: false, first: false });
    expect(pressed({ key: "j" }).defaultPrevented).toBe(true);
    pressed({ key: "ArrowDown" });
    expect(pressed({ key: "x" }).defaultPrevented).toBe(false);
    expect(got).toEqual(["next", "next"]);
  });

  it("does what you set in place of its own key", () => {
    const { got, h } = did();
    bind({ scope: scopeOf(list), handlers: () => ({ next: h("next") }), modal: false, first: false });
    setKeys(list.next, ["n"]);
    pressed({ key: "j" });
    pressed({ key: "n" });
    expect(got).toEqual(["next"]);
  });

  it("leaves one that isn't bound now, and one that gives back pass, to the next", () => {
    const { got, h } = did();
    bind({ scope: scopeOf(list), handlers: () => ({ next: () => pass, open: undefined }), modal: false, first: false });
    bind({ scope: scopeOf(list), handlers: () => ({ next: h("outer"), open: h("open") }), modal: false, first: false });
    pressed({ key: "j" });
    pressed({ key: "Enter" });
    expect(got).toEqual(["outer", "open"]);
  });

  it("takes the key from one that gives back false: it's done", () => {
    const { got, h } = did();
    bind({ scope: scopeOf(list), handlers: () => ({ next: () => false }), modal: false, first: false });
    bind({ scope: scopeOf(list), handlers: () => ({ next: h("outer") }), modal: false, first: false });
    expect(pressed({ key: "j" }).defaultPrevented).toBe(true);
    expect(got).toEqual([]);
  });

  it("asks one bound late after the rest, whenever it was bound: a page's own key is the page's", () => {
    const { got, h } = did();
    bind({ scope: scopeOf(list), handlers: () => ({ next: h("plugin") }), modal: false, first: false, late: true });
    bind({ scope: scopeOf(list), handlers: () => ({ next: h("page") }), modal: false, first: false });
    pressed({ key: "j" });
    expect(got).toEqual(["page"]);
  });

  it("does the one bound first: a part's before its page's", () => {
    const { got, h } = did();
    bind({ scope: scopeOf(list), handlers: () => ({ next: h("part") }), modal: false, first: false });
    bind({ scope: scopeOf(list), handlers: () => ({ next: h("page") }), modal: false, first: false });
    pressed({ key: "j" });
    expect(got).toEqual(["part"]);
  });

  it("does only a modal's while one's bound, and everywhere's first ones whatever's open", () => {
    const { got, h } = did();
    bind({ scope: scopeOf(list), handlers: () => ({ next: h("page"), prev: h("page prev") }), modal: false, first: false });
    const off = bind({ scope: scopeOf(list), handlers: () => ({ next: h("modal") }), modal: true, first: false });
    bind({ scope: scopeOf(go), handlers: () => ({ send: h("send") }), modal: false, first: true });
    pressed({ key: "j" });
    pressed({ key: "k" });
    pressed({ key: "Enter", metaKey: true });
    off();
    pressed({ key: "j" });
    expect(got).toEqual(["modal", "send", "page"]);
  });

  it("does one then another, and the first by itself as well", () => {
    const { got, h } = did();
    bind({ scope: scopeOf(go), handlers: () => ({ missions: h("missions") }), modal: false, first: true });
    bind({ scope: scopeOf(list), handlers: () => ({ first: h("first") }), modal: false, first: false });
    pressed({ key: "g" });
    pressed({ key: "m" });
    pressed({ key: "m" });
    expect(got).toEqual(["first", "missions"]);
  });

  it("does one then another you've set", () => {
    const { got, h } = did();
    bind({ scope: scopeOf(go), handlers: () => ({ missions: h("missions") }), modal: false, first: true });
    setKeys(go.missions, ["Space m"]);
    pressed({ key: "g" });
    pressed({ key: "m" });
    pressed({ key: " " });
    pressed({ key: "m" });
    expect(got).toEqual(["missions"]);
  });

  it("leaves a key alone that what has the focus took", () => {
    const { got, h } = did();
    bind({ scope: scopeOf(list), handlers: () => ({ next: h("next") }), modal: false, first: false });
    const e = key({ key: "j" });
    press(e, true);
    e.preventDefault();
    press(e, false);
    expect(got).toEqual([]);
  });

  it("gives every key to what's holding them, while it is", () => {
    const { got, h } = did();
    bind({ scope: scopeOf(go), handlers: () => ({ send: h("send") }), modal: false, first: true });
    const heard: string[] = [];
    const release = holdKeys((e) => heard.push(keyName(e)));
    expect(pressed({ key: "Enter", metaKey: true }).defaultPrevented).toBe(true);
    pressed({ key: "Shift" });
    release();
    pressed({ key: "Enter", metaKey: true });
    expect(heard).toEqual(["mod+Enter"]);
    expect(got).toEqual(["send"]);
  });

  it("doesn't do a fixed one: it's its part's", () => {
    const { got, h } = did();
    bind({ scope: scopeOf(list), handlers: () => ({ typed: h("typed") }), modal: false, first: false });
    pressed({ key: "```" });
    expect(got).toEqual([]);
  });
});

describe("the keymap", () => {
  it("is a command's own keys until you set others, and its own again once you take them back", () => {
    expect(keysOf(list.next)).toEqual(["j", "ArrowDown"]);
    setKeys(list.next, ["n"]);
    expect(keyOf(list.next)).toBe("n");
    expect(isOwn(list.next)).toBe(false);
    setKeys(list.next, ["j", "ArrowDown"]);
    expect(isOwn(list.next)).toBe(true);
    setKeys(list.next, []);
    expect(keyOf(list.next)).toBeUndefined();
    resetKeys(list.next);
    expect(keyOf(list.next)).toBe("j");
  });

  it("can't set a fixed one", () => {
    setKeys(list.typed, ["x"]);
    expect(keysOf(list.typed)).toEqual(["```"]);
  });

  it("says a clash only where it's yours", () => {
    expect(clashesOf(list.first, scopeOf(go))).toEqual([]);
    setKeys(list.open, ["j"]);
    expect(clashesOf(list.open, scopeOf(go)).map((c) => c.with.id)).toEqual(["test-list.next"]);
    expect(clashesOf(list.next, scopeOf(go)).map((c) => c.with.id)).toEqual(["test-list.open"]);
    setPref("keymap", { [go.send.id]: ["Enter"] });
    expect(clashesOf(list.open, scopeOf(go)).map((c) => c.with.id)).toEqual(["test-go.send"]);
  });
});

describe("a plugin's suggested keys", () => {
  const plugin = scope("plugin-test", "Test plugin", {
    note: { title: "New note", keys: [], suggested: ["n"] },
  });
  const other = scope("plugin-other", "Other plugin", {
    sketch: { title: "New sketch", keys: [], suggested: ["n"] },
  });

  it("aren't its keys until they're taken, then are yours", () => {
    expect(keysOf(plugin.note)).toEqual([]);
    takeSuggested(plugin.note);
    expect(keysOf(plugin.note)).toEqual(["n"]);
    expect(isOwn(plugin.note)).toBe(false);
  });

  it("clash, taken, with another plugin's taken, and with a page's, weighed against every scope", () => {
    takeSuggested(plugin.note);
    takeSuggested(other.sketch);
    expect(clashesOf(plugin.note, scopeOf(go), undefined, [scopeOf(other)]).map((c) => c.with.id)).toEqual(["plugin-other.sketch"]);
    setKeys(list.open, ["n"]);
    expect(clashesOf(plugin.note, scopeOf(go), undefined, [scopeOf(other), scopeOf(list)]).map((c) => c.with.id)).toEqual(["plugin-other.sketch", "test-list.open"]);
  });
});

describe("a clash of one then another", () => {
  it("is one you set that starts with another's key, either way round", () => {
    setKeys(list.open, ["j j"]);
    expect(clashesOf(list.open, scopeOf(go)).map((c) => [c.key, c.starts, c.with.id])).toEqual([["j j", "j", "test-list.next"]]);
    expect(clashesOf(list.next, scopeOf(go)).map((c) => [c.key, c.starts, c.with.id])).toEqual([["j j", "j", "test-list.open"]]);
  });
});

describe("the Keys dialog's lines", () => {
  it("are one a command, those that share a row together, the first key each", () => {
    expect(rowsOf(scopeOf(list).commands)).toEqual([
      ["j / k", "Next / previous"],
      ["g", "First"],
      ["Enter", "Open"],
      ["```", "Typed"],
    ]);
  });
  it("say digits one after another as a run", () => {
    const tabs = scope("test-tabs", "Tabs", {
      a: { title: "A", keys: "1", row: "A, B, C" },
      b: { title: "B", keys: "2", row: "A, B, C" },
      c: { title: "C", keys: "3", row: "A, B, C" },
    });
    expect(rowsOf(scopeOf(tabs).commands)).toEqual([["1–3", "A, B, C"]]);
    setKeys(tabs.b, ["x"]);
    expect(rowsOf(scopeOf(tabs).commands)).toEqual([["1 / x / 3", "A, B, C"]]);
  });

  it("say your keys", () => {
    setKeys(list.prev, ["p"]);
    expect(rowsOf(scopeOf(list).commands)[0]).toEqual(["j / p", "Next / previous"]);
  });
});

describe("the keys a plugin's page passes on", () => {
  it("are everywhere's that say frame, as they are now", () => {
    const k = frameKeys();
    expect(new Set(k.chords)).toEqual(new Set(["k", "p", "j"]));
    expect(new Set(k.bare)).toEqual(new Set(["?", "C", "g"]));
    expect(k.leaders).toEqual(["g"]);
    setKeys(everywhereKeys.find, ["mod+e"]);
    setKeys(everywhereKeys.help, ["mod+shift+h"]);
    expect(frameKeys().chords).toContain("e");
    expect(frameKeys().chords).not.toContain("k");
    expect(frameKeys().bare).not.toContain("?");
  });
});
