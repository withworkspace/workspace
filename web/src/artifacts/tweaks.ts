// Tweaks, apart from how they're drawn: what an artifact's page says can
// be changed about it (ws.tweaks, which the server writes into every
// frame: internal/web/tweaks.js), as the page around its frame reads it,
// and the state as the address keeps it.
//
// A frame is a page an agent wrote, in an origin of its own. What it says
// is taken as data and nothing more: every part is checked here before
// it's drawn, and what doesn't fit is dropped.

export type Value = string | number | boolean;

// State is what each tweak is set to, by its name.
export type State = Record<string, Value>;

export type Kind = "choice" | "toggle" | "number" | "text" | "color";

export interface Choice {
  value: string;
  label: string;
  description: string;
}

export interface Tweak {
  name: string;
  label: string;
  description: string;
  kind: Kind;
  options?: Choice[]; // a choice's
  min?: number; // a number's
  max?: number;
  step?: number;
  unit?: string;
}

// Preset sets several tweaks at once: the ones it lists, the rest left as
// they are. The first is what the page opens as, and lists them all.
export interface Preset {
  label: string;
  description: string;
  state: State;
}

// Told is what a frame's page declared, and where it stands: its tweaks
// in order, its presets, the state it opens as and the state it's in.
export interface Told {
  tweaks: Tweak[];
  presets: Preset[];
  opening: State;
  state: State;
  problems: string[]; // what was wrong with the declaration, for whoever wrote it
}

const MOST = { tweaks: 40, presets: 20, options: 30, text: 200, label: 80, description: 300, problems: 20 };
const NAME = /^[A-Za-z][A-Za-z0-9_-]{0,39}$/;
const COLOR = /^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/;
const UNIT = /^[A-Za-z%]{0,8}$/;
const KINDS: Kind[] = ["choice", "toggle", "number", "text", "color"];

const isObject = (v: unknown): v is Record<string, unknown> => v !== null && typeof v === "object" && !Array.isArray(v);
const words = (v: unknown, most: number) => (typeof v === "string" ? v.slice(0, most) : "");
const finite = (v: unknown) => (typeof v === "number" && Number.isFinite(v) ? v : undefined);

// takes is whether v is a value the tweak can be set to.
export function takes(t: Tweak, v: unknown): v is Value {
  switch (t.kind) {
    case "choice":
      return typeof v === "string" && !!t.options?.some((o) => o.value === v);
    case "toggle":
      return typeof v === "boolean";
    case "number":
      return typeof v === "number" && Number.isFinite(v) && (t.min === undefined || v >= t.min) && (t.max === undefined || v <= t.max);
    case "color":
      return typeof v === "string" && COLOR.test(v);
    case "text":
      return typeof v === "string" && v.length <= MOST.text;
  }
}

function readTweak(d: unknown): Tweak | undefined {
  if (!isObject(d) || typeof d.name !== "string" || !NAME.test(d.name) || !KINDS.includes(d.kind as Kind)) return undefined;
  const t: Tweak = { name: d.name, label: words(d.label, MOST.label) || d.name, description: words(d.description, MOST.description), kind: d.kind as Kind };
  if (t.kind === "choice") {
    const options: Choice[] = [];
    for (const o of Array.isArray(d.options) ? d.options.slice(0, MOST.options) : []) {
      if (!isObject(o) || typeof o.value !== "string" || !o.value || o.value.length > MOST.text || options.some((x) => x.value === o.value)) continue;
      options.push({ value: o.value, label: words(o.label, MOST.label) || o.value, description: words(o.description, MOST.description) });
    }
    if (!options.length) return undefined;
    t.options = options;
  }
  if (t.kind === "number") {
    const min = finite(d.min);
    const max = finite(d.max);
    if (min === undefined || max === undefined || min <= max) {
      if (min !== undefined) t.min = min;
      if (max !== undefined) t.max = max;
    }
    const step = finite(d.step);
    if (step !== undefined && step > 0) t.step = step;
    if (typeof d.unit === "string" && UNIT.test(d.unit)) t.unit = d.unit;
  }
  return t;
}

// readState keeps, of what a frame says is set, the tweaks there are and
// the values they take.
function readState(tweaks: Tweak[], d: unknown): State {
  const out: State = {};
  if (!isObject(d)) return out;
  for (const t of tweaks) {
    const v = d[t.name];
    if (takes(t, v)) out[t.name] = v;
  }
  return out;
}

// readTold reads what a frame's page declared, from its message. Nothing,
// for a message that isn't that, or that declares what can't be drawn (the
// opening state has to say every tweak).
export function readTold(d: unknown): Told | undefined {
  if (!isObject(d) || d.ws !== "tweaks" || !Array.isArray(d.tweaks)) return undefined;
  const tweaks: Tweak[] = [];
  for (const raw of d.tweaks.slice(0, MOST.tweaks)) {
    const t = readTweak(raw);
    if (t && !tweaks.some((x) => x.name === t.name)) tweaks.push(t);
  }
  const opening = readState(tweaks, d.opening);
  if (tweaks.some((t) => !(t.name in opening))) return undefined;
  const presets: Preset[] = [];
  for (const p of Array.isArray(d.presets) ? d.presets.slice(0, MOST.presets) : []) {
    if (!isObject(p)) continue;
    presets.push({ label: words(p.label, MOST.label) || `Preset ${presets.length + 1}`, description: words(p.description, MOST.description), state: readState(tweaks, p.state) });
  }
  const problems = (Array.isArray(d.problems) ? d.problems : [])
    .filter((p): p is string => typeof p === "string")
    .slice(0, MOST.problems)
    .map((p) => p.slice(0, MOST.description));
  return { tweaks, presets, opening, state: { ...opening, ...readState(tweaks, d.state) }, problems };
}

// readChange reads a frame's message that its state changed, as the state
// it's in now: nothing, for a message that isn't that.
export function readChange(told: Pick<Told, "tweaks" | "state">, d: unknown): State | undefined {
  if (!isObject(d) || d.ws !== "tweaks:state") return undefined;
  return { ...told.state, ...readState(told.tweaks, d.state) };
}

// presetOn is whether the state is as a preset sets it: every tweak it
// lists, at the value it gives.
export const presetOn = (p: Preset, state: State) => Object.entries(p.state).every(([name, v]) => state[name] === v);

// changed are the tweaks that aren't as the page opens them.
export const changed = (told: Pick<Told, "tweaks" | "opening" | "state">) => told.tweaks.filter((t) => told.state[t.name] !== told.opening[t.name]);

// written is a value as the address has it, and as the frame's script
// reads it there: on and off for a toggle.
export const written = (v: Value) => (typeof v === "boolean" ? (v ? "on" : "off") : String(v));

// said is a value as a person reads it: a choice by its label, a number
// with its unit, a toggle on or off.
export function said(t: Tweak, v: Value): string {
  if (t.kind === "choice") return t.options?.find((o) => o.value === v)?.label ?? String(v);
  if (t.kind === "number") return `${v}${t.unit ?? ""}`;
  return written(v);
}

// stateWords is the state in lines, a tweak each, to paste to whoever
// made the page: "Density: Compact".
export const stateWords = (told: Pick<Told, "tweaks" | "state">) => told.tweaks.map((t) => `${t.label}: ${said(t, told.state[t.name])}`).join("\n");

// --- The address
//
// A page's address says the state where it isn't the opening one:
// ?t.density=compact&t.pips=off. The frame's own address says the same,
// and its script reads it, so a frame opens in the state its page is in.
// Comparing has two frames: a. and b. for the left and the right.

export const TWEAK = "t.";

// Params are a state as an address writes it: each tweak's name and its
// value, as written.
export type Params = [name: string, value: string][];

// tweakParams is what the address says of a frame's state: the tweaks
// that aren't as the page opens them.
export const tweakParams = (told: Pick<Told, "tweaks" | "opening" | "state">): Params => changed(told).map((t) => [t.name, written(told.state[t.name])]);

// readTweakParams reads the state an address gives, under a prefix, as
// it's written: what each is worth is the frame's to say.
export function readTweakParams(search: URLSearchParams, prefix = TWEAK): Params {
  const out: Params = [];
  for (const [key, value] of search) if (key.startsWith(prefix) && key.length > prefix.length) out.push([key.slice(prefix.length), value]);
  return out;
}

// withTweakParams is an address's search with the state under a prefix
// put in place of what was there.
export function withTweakParams(search: URLSearchParams, params: Params, prefix = TWEAK): URLSearchParams {
  const out = new URLSearchParams();
  for (const [key, value] of search) if (!key.startsWith(prefix)) out.append(key, value);
  for (const [name, value] of params) out.append(prefix + name, value);
  return out;
}

// followed is an address's search once it says the state each frame is
// in, under that frame's prefix: nothing, where it says so already. A
// frame that hasn't said what it declares (told is undefined) keeps what
// the address has for it: that's the state it's opening in.
export function followed(search: URLSearchParams, frames: [prefix: string, told: Pick<Told, "tweaks" | "opening" | "state"> | undefined][]): string | undefined {
  let next = search;
  let moved = false;
  for (const [prefix, told] of frames) {
    if (!told) continue;
    const now = tweakParams(told);
    const had = readTweakParams(search, prefix);
    if (now.length === had.length && now.every(([name, value], i) => had[i][0] === name && had[i][1] === value)) continue;
    next = withTweakParams(next, now, prefix);
    moved = true;
  }
  // A comma's written as one: v=1,3 stays as it reads.
  return moved ? next.toString().replace(/%2C/g, ",") : undefined;
}

// frameSearch is a state as a frame's address has it: "?t.density=compact",
// or nothing.
export function frameSearch(params: Params): string {
  const s = withTweakParams(new URLSearchParams(), params).toString();
  return s ? `?${s}` : "";
}
