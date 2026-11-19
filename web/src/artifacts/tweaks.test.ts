import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { compareHref } from "./pair";
import { changed, followed, frameSearch, presetOn, readChange, readTold, readTweakParams, said, stateWords, takes, tweakParams, withTweakParams, type Told } from "./tweaks";

// The script the server writes into every frame, run here as a frame runs
// it: against a page that's only what the script touches (its <html>, its
// address, the page that frames it).
const script = readFileSync(new URL("../../../internal/web/tweaks.js", import.meta.url), "utf8");

interface Page {
  ws: { tweaks: ((d: unknown, onChange?: (s: Record<string, unknown>) => void) => Record<string, unknown>) & { set: (s: unknown) => void; state: Record<string, unknown> } };
  attrs: Map<string, string>;
  vars: Map<string, string>;
  posted: Record<string, unknown>[]; // what it said to the page around it
  events: unknown[]; // ws:tweaks, as the page hears them
  tell: (data: unknown, from?: unknown) => void; // a message to it
}

function page(search = "", alone = false): Page {
  const attrs = new Map<string, string>();
  const vars = new Map<string, string>();
  const posted: Record<string, unknown>[] = [];
  const events: unknown[] = [];
  const heard: ((e: unknown) => void)[] = [];
  const host = { postMessage: (m: Record<string, unknown>) => posted.push(structuredClone(m)) };
  const window: Record<string, unknown> = {
    addEventListener: (type: string, fn: (e: unknown) => void) => type === "message" && heard.push(fn),
    dispatchEvent: (e: { detail: unknown }) => events.push(e.detail),
  };
  window.parent = alone ? window : host;
  const document = {
    documentElement: {
      setAttribute: (k: string, v: string) => attrs.set(k, v),
      removeAttribute: (k: string) => attrs.delete(k),
      style: { setProperty: (k: string, v: string) => vars.set(k, v), removeProperty: (k: string) => vars.delete(k) },
    },
  };
  class CustomEvent {
    detail: unknown;
    constructor(_: string, init: { detail: unknown }) {
      this.detail = init.detail;
    }
  }
  new Function("window", "document", "location", "CustomEvent", script)(window, document, { search }, CustomEvent);
  return { ws: window.ws as Page["ws"], attrs, vars, posted, events, tell: (data, from = host) => heard.forEach((fn) => fn({ source: from, data })) };
}

// A plan list's prototype, as an agent would declare it.
const plan = {
  tweaks: {
    density: {
      label: "Density",
      description: "Row height in the plan list.",
      options: ["Cozy rows", { value: "compact", label: "Compact", description: "32px rows" }],
    },
    pips: { label: "Progress pips" },
    radius: { label: "Corner radius", min: 0, max: 16, unit: "px" },
    accent: { label: "Accent", type: "color" },
    heading: { label: "Heading" },
  },
  presets: [
    { label: "As it is today", description: "What ships now.", state: { density: "Cozy rows", pips: true, radius: 8, accent: "#5B8DEF", heading: "Plan" } },
    { label: "Suggested", state: { density: "compact", radius: 12 } },
    { label: "No pips", state: { pips: false } },
  ],
};

const opening = { density: "Cozy rows", pips: true, radius: 8, accent: "#5b8def", heading: "Plan" };

describe("ws.tweaks, in a frame", () => {
  it("opens as the first preset says, and writes it on <html> for the page's styles", () => {
    const p = page();
    const seen: unknown[] = [];
    expect(p.ws.tweaks(plan, (s) => seen.push(s))).toEqual(opening);
    expect(Object.fromEntries(p.attrs)).toEqual({ "data-density": "Cozy rows", "data-pips": "", "data-radius": "8", "data-accent": "#5b8def", "data-heading": "Plan" });
    expect(Object.fromEntries(p.vars)).toEqual({ "--radius": "8px", "--accent": "#5b8def" });
    // The page hears the state once as it declares, to draw from one place.
    expect(seen).toEqual([opening]);
    expect(p.events).toEqual([opening]);
    expect(p.ws.tweaks.state).toEqual(opening);
  });

  it("tells the page around it what's declared: each tweak's kind, the presets, and the state", () => {
    const p = page();
    p.ws.tweaks(plan);
    expect(p.posted).toHaveLength(1);
    const told = p.posted[0] as unknown as Told & { ws: string };
    expect(told.ws).toBe("tweaks");
    expect(told.tweaks.map((t) => [t.name, t.kind])).toEqual([
      ["density", "choice"],
      ["pips", "toggle"],
      ["radius", "number"],
      ["accent", "color"],
      ["heading", "text"],
    ]);
    expect(told.tweaks[0].options).toEqual([
      { value: "Cozy rows", label: "Cozy rows", description: "" },
      { value: "compact", label: "Compact", description: "32px rows" },
    ]);
    expect(told.tweaks[2]).toMatchObject({ min: 0, max: 16, unit: "px" });
    expect(told.presets.map((x) => [x.label, x.state])).toEqual([
      ["As it is today", opening],
      ["Suggested", { density: "compact", radius: 12 }],
      ["No pips", { pips: false }],
    ]);
    expect(told.opening).toEqual(opening);
    expect(told.state).toEqual(opening);
    expect(told.problems).toEqual([]);
  });

  it("opens in the state its address gives, where the page takes it", () => {
    const p = page("?t.density=compact&t.pips=off&t.radius=40&t.nope=1&t.accent=red&other=1");
    // A radius out of range, a colour that isn't one and a tweak that isn't
    // there are dropped: those open as the page says.
    expect(p.ws.tweaks(plan)).toEqual({ ...opening, density: "compact", pips: false });
    expect(p.attrs.get("data-density")).toBe("compact");
    expect(p.attrs.has("data-pips")).toBe(false);
    expect(p.vars.get("--radius")).toBe("8px");
    expect((p.posted[0] as unknown as Told).opening).toEqual(opening);
  });

  it("is set from the page around it, and says the state it's then in", () => {
    const p = page();
    const seen: unknown[] = [];
    p.ws.tweaks(plan, (s) => seen.push(s));
    p.tell({ ws: "tweaks:set", state: { density: "compact", radius: 12 } });
    const next = { ...opening, density: "compact", radius: 12 };
    expect(p.posted[1]).toEqual({ ws: "tweaks:state", state: next });
    expect(seen[1]).toEqual(next);
    expect(p.events[1]).toEqual(next);
    expect(p.attrs.get("data-density")).toBe("compact");
    expect(p.vars.get("--radius")).toBe("12px");
    // A toggle off takes its attribute away.
    p.tell({ ws: "tweaks:set", state: { pips: false } });
    expect(p.attrs.has("data-pips")).toBe(false);
    // What it doesn't take, and what's as it is already, changes nothing.
    p.tell({ ws: "tweaks:set", state: { density: "roomy", radius: 99, pips: false, nope: 1 } });
    expect(p.posted).toHaveLength(3);
    // Nor does a message from anywhere but the page that frames it.
    p.tell({ ws: "tweaks:set", state: { density: "Cozy rows" } }, {});
    expect(p.posted).toHaveLength(3);
    // Asked, it says what it declared again, with the state it's in.
    p.tell({ ws: "tweaks:hello" });
    expect(p.posted[3]).toMatchObject({ ws: "tweaks", state: { ...next, pips: false } });
  });

  it("can be set by the page itself", () => {
    const p = page();
    p.ws.tweaks(plan);
    p.ws.tweaks.set({ heading: "Steps" });
    expect(p.posted[1]).toEqual({ ws: "tweaks:state", state: { ...opening, heading: "Steps" } });
  });

  it("says what's wrong with a declaration, and draws what it can of it", () => {
    const p = page();
    p.ws.tweaks({
      tweaks: {
        "2col": { label: "Two columns" },
        mystery: { label: "No telling what" },
        empty: { options: [] },
        size: { options: ["s", "m", "m", 3] },
        gap: { min: 8, max: 4 },
        shown: {},
      },
      presets: [{ state: { size: "l", shown: true, nope: 1 } }, "not a preset"],
    });
    const told = p.posted[0] as unknown as Told;
    expect(told.tweaks.map((t) => [t.name, t.kind])).toEqual([
      ["size", "choice"],
      ["gap", "number"],
      ["shown", "toggle"],
    ]);
    // The first preset is made to say every tweak.
    expect(told.opening).toEqual({ size: "s", gap: 0, shown: true });
    expect(told.presets.map((x) => x.label)).toEqual(["As it opens", "Preset 2"]);
    expect(told.problems).toEqual([
      "2col: a tweak's name is letters, digits, - and _, starting with a letter",
      "mystery: the first preset gives it no value, so there's no telling what it is",
      "empty: it has no options",
      "size: m is there twice",
      "size: an option is a string, or { value, label }",
      "gap: its min is over its max",
      'As it opens: size can\'t be "l"',
      "As it opens: there's no tweak named nope",
      "As it opens: it gives size no value",
      "As it opens: it gives gap no value",
    ]);
  });

  it("has a preset to open as, where the page gives none", () => {
    const p = page();
    p.ws.tweaks({ tweaks: { size: { options: ["s", "m"] } } });
    const told = p.posted[0] as unknown as Told;
    expect(told.presets).toEqual([{ label: "As it opens", description: "", state: { size: "s" } }]);
    expect(told.problems).toEqual(["there are no presets: the first is what the page opens as"]);
  });

  it("declared again, takes away what the page no longer has", () => {
    const p = page();
    p.ws.tweaks(plan);
    p.ws.tweaks({ tweaks: { pips: {} }, presets: [{ state: { pips: false } }] });
    expect(Object.fromEntries(p.attrs)).toEqual({});
    expect(Object.fromEntries(p.vars)).toEqual({});
  });

  it("works with no page around it: opened alone, the address is the state", () => {
    const p = page("?t.radius=4", true);
    expect(p.ws.tweaks(plan)).toEqual({ ...opening, radius: 4 });
    p.ws.tweaks.set({ radius: 6 });
    expect(p.vars.get("--radius")).toBe("6px");
    expect(p.posted).toEqual([]);
  });
});

// What the frame says, as the page around it reads it.
function told(search = ""): Told {
  const p = page(search);
  p.ws.tweaks(plan);
  const t = readTold(p.posted[0]);
  if (!t) throw new Error("the frame's message wasn't read");
  return t;
}

describe("readTold", () => {
  it("reads what the frame's script says, whole", () => {
    const t = told("?t.density=compact");
    expect(t.tweaks.map((x) => x.name)).toEqual(["density", "pips", "radius", "accent", "heading"]);
    expect(t.opening).toEqual(opening);
    expect(t.state).toEqual({ ...opening, density: "compact" });
    expect(t.presets).toHaveLength(3);
  });

  it("takes a frame's word for nothing: what doesn't fit is dropped", () => {
    const t = readTold({
      ws: "tweaks",
      tweaks: [
        { name: "size", kind: "choice", label: 7, options: [{ value: "s" }, { value: "s" }, "m", { value: "" }] },
        { name: "size", kind: "toggle" },
        { name: "on click", kind: "toggle" },
        { name: "sound", kind: "volume" },
        { name: "none", kind: "choice", options: [] },
        { name: "gap", kind: "number", min: 8, max: 4, step: -1, unit: "px;color:red" },
        "nonsense",
      ],
      presets: [{ label: "x".repeat(200), state: { size: "xl", gap: 2, nope: true } }, 4],
      opening: { size: "s", gap: 0 },
      state: { size: "xl", gap: "2" },
      problems: ["one", 2, "y".repeat(900)],
    });
    expect(t?.tweaks).toEqual([
      { name: "size", label: "size", description: "", kind: "choice", options: [{ value: "s", label: "s", description: "" }] },
      { name: "gap", label: "gap", description: "", kind: "number" },
    ]);
    expect(t?.presets).toEqual([{ label: "x".repeat(80), description: "", state: { gap: 2 } }]);
    // A state that isn't one the tweaks take is the opening one.
    expect(t?.state).toEqual({ size: "s", gap: 0 });
    expect(t?.problems).toEqual(["one", "y".repeat(300)]);
  });

  it("is nothing for another message, or an opening state that leaves a tweak out", () => {
    expect(readTold(undefined)).toBeUndefined();
    expect(readTold({ ws: "tweaks:state", state: {} })).toBeUndefined();
    expect(readTold({ ws: "tweaks", tweaks: [{ name: "on", kind: "toggle" }], opening: {} })).toBeUndefined();
    expect(readTold({ ws: "tweaks", tweaks: [], opening: {} })).toEqual({ tweaks: [], presets: [], opening: {}, state: {}, problems: [] });
  });
});

describe("the state", () => {
  it("follows the frame's changes, keeping to what the tweaks take", () => {
    const t = told();
    expect(readChange(t, { ws: "tweaks:state", state: { density: "compact", radius: 99, nope: 1 } })).toEqual({ ...opening, density: "compact" });
    expect(readChange(t, { ws: "tweaks", state: {} })).toBeUndefined();
  });

  it("says which tweaks aren't as the page opens them, and which presets are on", () => {
    const t = told("?t.density=compact&t.radius=12");
    expect(changed(t).map((x) => x.name)).toEqual(["density", "radius"]);
    expect(t.presets.map((p) => presetOn(p, t.state))).toEqual([false, true, false]);
    expect(told().presets.map((p) => presetOn(p, opening))).toEqual([true, false, false]);
  });

  it("says a value as a person reads it", () => {
    const t = told("?t.density=compact&t.pips=off");
    expect(t.tweaks.map((x) => said(x, t.state[x.name]))).toEqual(["Compact", "off", "8px", "#5b8def", "Plan"]);
    expect(stateWords(t)).toBe("Density: Compact\nProgress pips: off\nCorner radius: 8px\nAccent: #5b8def\nHeading: Plan");
  });

  it("knows what each tweak takes", () => {
    const [density, pips, radius, accent, heading] = told().tweaks;
    expect([takes(density, "compact"), takes(density, "Compact"), takes(density, 1)]).toEqual([true, false, false]);
    expect([takes(pips, true), takes(pips, "on")]).toEqual([true, false]);
    expect([takes(radius, 16), takes(radius, 16.5), takes(radius, NaN), takes(radius, "8")]).toEqual([true, false, false, false]);
    expect([takes(accent, "#abc"), takes(accent, "#abcd"), takes(accent, "red")]).toEqual([true, false, false]);
    expect([takes(heading, ""), takes(heading, "x".repeat(201))]).toEqual([true, false]);
  });
});

describe("the address", () => {
  const q = (s: string) => new URLSearchParams(s);

  it("says the state where it isn't the opening one, and nothing where it is", () => {
    expect(tweakParams(told())).toEqual([]);
    expect(tweakParams(told("?t.density=compact&t.pips=off&t.radius=12"))).toEqual([
      ["density", "compact"],
      ["pips", "off"],
      ["radius", "12"],
    ]);
  });

  it("is read back as it's written, under its prefix", () => {
    expect(readTweakParams(q("v=1,2&t.density=Cozy+rows&t.=x&a.pips=off"))).toEqual([["density", "Cozy rows"]]);
    expect(readTweakParams(q("a.pips=off&b.pips=on"), "b.")).toEqual([["pips", "on"]]);
  });

  it("puts a state in place of the one that was there, leaving the rest", () => {
    expect(withTweakParams(q("v=1,2&t.density=compact&t.pips=off"), [["radius", "12"]]).toString()).toBe("v=1%2C2&t.radius=12");
    expect(withTweakParams(q("t.density=compact"), []).toString()).toBe("");
    expect(withTweakParams(q("a.x=1&b.x=2"), [["x", "3"]], "b.").toString()).toBe("a.x=1&b.x=3");
  });

  it("follows each frame's state, and is left alone where it says it already", () => {
    const compact = told("?t.density=compact");
    // Nothing to change: the address says it, or the frame hasn't said yet.
    expect(followed(q("t.density=compact"), [["t.", compact]])).toBeUndefined();
    expect(followed(q("t.density=compact"), [["t.", undefined]])).toBeUndefined();
    expect(followed(q(""), [["t.", told()]])).toBeUndefined();
    // The frame's state in place of what was there, the rest as it reads.
    expect(followed(q("v=1,3&t.pips=off"), [["t.", compact]])).toBe("v=1,3&t.density=compact");
    expect(followed(q("t.density=compact"), [["t.", told()]])).toBe("");
    // Two frames, each under its own prefix: one that hasn't said keeps its own.
    expect(
      followed(q("v=2,2&a.pips=off&b.radius=4"), [
        ["a.", undefined],
        ["b.", compact],
      ]),
    ).toBe("v=2,2&a.pips=off&b.density=compact");
  });

  it("is the frame's address too", () => {
    expect(frameSearch([])).toBe("");
    expect(frameSearch([["density", "Cozy rows"]])).toBe("?t.density=Cozy+rows");
    // What it writes, the frame's script reads.
    const p = page(frameSearch(tweakParams(told("?t.density=compact&t.heading=A+%26+B"))));
    expect(p.ws.tweaks(plan)).toEqual({ ...opening, density: "compact", heading: "A & B" });
  });

  it("keeps each side's state in a comparison's address", () => {
    const pair: [{ id: string; n: number }, { id: string; n: number }] = [
      { id: "a3", n: 2 },
      { id: "a3", n: 2 },
    ];
    expect(compareHref(pair, [[], [["density", "compact"]]])).toBe("/artifacts/a3/compare?v=2,2&b.density=compact");
    expect(compareHref(pair, [[], []])).toBe("/artifacts/a3/compare?v=2,2");
    expect(
      compareHref(
        [
          { id: "a3", n: 0 },
          { id: "a3", n: 0 },
        ],
        [[["pips", "off"]], []],
      ),
    ).toBe("/artifacts/a3/compare?a.pips=off");
  });
});
