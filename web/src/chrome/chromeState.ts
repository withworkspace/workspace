// How one window is arranged: where the sidebar is, what's beside the
// page and what's below it. It isn't the daemon's: it's kept in the browser for the
// workspace, as the theme and the widths are, and two windows may differ.

// Where the sidebar is: open, folded to a rail, or hidden. Hidden, none
// of it is drawn: you get about by the keys (Find, the places' own), and
// the foot's toggle brings it back. On a narrow window it's a drawer: out
// of sight until that toggle opens it over the page.
export type SidebarPlace = "open" | "rail" | "hidden" | "drawer";

// Under this width a window is narrow: there's no room for a sidebar
// beside the page.
export const narrowBelow = 860;
export const narrowQuery = `(max-width: ${narrowBelow}px)`;

// From this width a window has room for Beside next to the page, docked:
// the open sidebar's column (244), Beside as it rests (420) and its inset,
// and about 880 left for the page. It's the window's width alone, not
// what the page is doing (a rail on a thing's page, the sidebar open on a
// list), so going from page to page never moves it. A 13" or 14" laptop
// is under it and Beside floats over the page; a 16" and a desktop's
// screen dock it. Dragged wider than it rests, it takes from the page and
// doesn't float.
export const docksFrom = 1560;
export const docksQuery = `(min-width: ${docksFrom}px)`;

// From this width a page that wants the room has it with the sidebar
// open, and Beside docked too: the sidebar's column (244), Beside as it
// rests (432 with its inset), and over 1,200 left for a diff, the files'
// explorer, a mission. So the sidebar stays open on every page and going
// from a list to a thing doesn't move the page. A 16" laptop is under it
// (Beside docks, the sidebar still folds where a page wants the room); a
// desktop's screen is over it.
export const keepsSidebarFrom = 1920;
export const keepsSidebarQuery = `(min-width: ${keepsSidebarFrom}px)`;

export interface ChromeState {
  // The sidebar is hidden. It's yours to choose and holds until you undo
  // it; a page never does it.
  hidden: boolean;
  // Beside: a peek at a second thing, next to the page.
  beside: {
    open: boolean;
    // Over the page, not next to it: the page keeps its width. It's
    // yours to choose where the window has room for it docked; where it
    // hasn't, it's over the page whatever this says.
    over: boolean;
    width: number;
    // What's open there, each by its address, in the order opened.
    tabs: string[];
    // What a tab is about, for those that are about something: the thing
    // of the page it was opened from (Find's Here, by id). It's there only
    // while the page is of that thing: what you asked about a document is
    // beside that document. One with nothing here stays as you go from
    // page to page.
    about: Record<string, string>;
    // Which is in front.
    front?: string;
    // The tab that's a peek (z, on what the pointer's on): there's one,
    // and the next thing peeked at takes its place, so looking at one row
    // after another leaves one tab and not a row of them.
    peek?: string;
    // Which was in front on each thing's page ("" where the page is of
    // nothing): coming back to a thing, what was in front there is again.
    fronts: Record<string, string>;
  };
  // Below: what's going on while you're on the page, under it. Its tabs
  // are views (what's running, what it came to), which whoever draws it
  // has: this keeps only which was showing.
  below: {
    open: boolean;
    height: number;
    // The view that's showing, by its id. None, or one that's no longer
    // there: the first.
    tab?: string;
    // Which of it: what's of the thing this page is of, or the
    // workspace's.
    where: BelowWhere;
    // When it was last closed (ms): what's started since is new to you,
    // and its toggle says so. None until it's first been seen.
    seen?: number;
  };
}

// Which of what's below is shown: Find's words for the same question.
export type BelowWhere = "here" | "everywhere";

// How wide Beside may be, and what a double-click on its edge puts it
// back to.
export const besideWidths = { min: 300, max: 760, rest: 420 };

// How tall Below may be, and what a double-click on its edge puts it back
// to. On a short window the shell holds it to less (Shell's --dock-most).
export const belowHeights = { min: 140, max: 620, rest: 240 };

// A window nobody has arranged.
export const resting: ChromeState = {
  hidden: false,
  beside: { open: false, over: false, width: besideWidths.rest, tabs: [], about: {}, fronts: {} },
  below: { open: false, height: belowHeights.rest, where: "here" },
};

// sidebarPlace is where the sidebar is drawn. A drawer on a narrow
// window, whatever else. Nowhere if you've hidden it; otherwise the page
// says, as it has: a page that wants the room gets the rail, unless the
// window's wide enough that it has the room anyway (keepsSidebarFrom).
export function sidebarPlace(s: ChromeState, wantsRoom: boolean, narrow = false, wide = false): SidebarPlace {
  if (narrow) return "drawer";
  return s.hidden ? "hidden" : wantsRoom && !wide ? "rail" : "open";
}

export const hideSidebar = (s: ChromeState, hidden: boolean): ChromeState => ({ ...s, hidden });

const has = (o: object, k: string) => Object.prototype.hasOwnProperty.call(o, k);

// besideTabs is what's beside the page that's of here, in the order it's
// drawn: what you opened about this page's thing, which is there only
// here, then what's about nothing, which is there anywhere.
export function besideTabs(s: ChromeState, here?: string): { here: string[]; anywhere: string[]; all: string[] } {
  const about = (t: string) => (has(s.beside.about, t) ? s.beside.about[t] : undefined);
  const mine = s.beside.tabs.filter((t) => here !== undefined && about(t) === here);
  const anywhere = s.beside.tabs.filter((t) => about(t) === undefined);
  return { here: mine, anywhere, all: [...mine, ...anywhere] };
}

// besideFront is the tab in front: what was in front on this thing's page
// if it's still there, or the one last picked anywhere, or the first.
export function besideFront(s: ChromeState, here?: string): string | undefined {
  const tabs = besideTabs(s, here).all;
  return [s.beside.fronts[here ?? ""], s.beside.front].find((t) => t !== undefined && tabs.includes(t)) ?? tabs[0];
}

// besideShown is whether Beside is drawn: it's open, and has something in
// it here.
export const besideShown = (s: ChromeState, here?: string) => s.beside.open && besideTabs(s, here).all.length > 0;

// besideAbout is what a tab is about, if anything.
export const besideAbout = (s: ChromeState, ref: string): string | undefined => (has(s.beside.about, ref) ? s.beside.about[ref] : undefined);

// How many things' pages are remembered for what was in front there.
const frontsKept = 60;

const frontAt = (fronts: Record<string, string>, here: string | undefined, ref: string): Record<string, string> => {
  const { [here ?? ""]: _, ...rest } = fronts;
  const next = { ...rest, [here ?? ""]: ref };
  const keys = Object.keys(next);
  for (const k of keys.slice(0, Math.max(0, keys.length - frontsKept))) delete next[k];
  return next;
};

const without = (o: Record<string, string>, k: string | undefined): Record<string, string> => {
  if (k === undefined || !has(o, k)) return o;
  const { [k]: _, ...rest } = o;
  return rest;
};

// openBeside opens a thing beside, in front: a new tab if it isn't there.
// about: the thing it's about (an ask, about the page it was asked on),
// which makes a new tab one that's there only on that thing's page; with
// none, it's there anywhere. here: where you are, which remembers it in
// front. inPlaceOf: it takes that tab's place if that's there (a
// conversation that's begun, where the one about to begin was), and that
// tab goes either way.
export function openBeside(s: ChromeState, ref: string, o: { inPlaceOf?: string; about?: string; here?: string } = {}): ChromeState {
  const was = o.inPlaceOf !== undefined && o.inPlaceOf !== ref && s.beside.tabs.includes(o.inPlaceOf) ? o.inPlaceOf : undefined;
  const isNew = !s.beside.tabs.includes(ref);
  let tabs = s.beside.tabs;
  if (!isNew) tabs = tabs.filter((t) => t !== was);
  else tabs = was ? tabs.map((t) => (t === was ? ref : t)) : [...tabs, ref];
  let about = without(s.beside.about, was);
  if (isNew && o.about !== undefined) about = { ...about, [ref]: o.about };
  return { ...s, beside: { ...s.beside, open: true, tabs, about, front: ref, fronts: frontAt(s.beside.fronts, o.here, ref) } };
}

// pickBeside brings a tab to the front, here.
export const pickBeside = (s: ChromeState, ref: string, here?: string): ChromeState => ({
  ...s,
  beside: { ...s.beside, front: ref, fronts: frontAt(s.beside.fronts, here, ref) },
});

// closeBesideTab closes a tab. If it was in front, the one after it comes
// forward (or the one before, at the end). Beside stays open: with
// nothing left to show here it isn't drawn, and what's beside another
// thing's page is still there when you get to it.
export function closeBesideTab(s: ChromeState, ref: string, here?: string): ChromeState {
  if (!s.beside.tabs.includes(ref)) return s;
  const before = besideTabs(s, here).all;
  const front = besideFront(s, here);
  const left = before.filter((t) => t !== ref);
  const i = before.indexOf(ref);
  const next = front === ref ? (left[i] ?? left[i - 1]) : front;
  const fronts = Object.fromEntries(Object.entries(s.beside.fronts).filter(([, t]) => t !== ref));
  return {
    ...s,
    beside: { ...s.beside, tabs: s.beside.tabs.filter((t) => t !== ref), about: without(s.beside.about, ref), front: next, fronts: next === undefined ? fronts : frontAt(fronts, here, next), peek: s.beside.peek === ref ? undefined : s.beside.peek },
  };
}

// peekBeside is z on a thing: it opens beside, in front, as the peek, in
// the place of what was peeked at before. On the peek that's in front
// already, it closes it: z opens and z closes. A thing that's open beside
// as a tab of its own (a conversation you asked for) comes to the front
// and stays one of yours, and z on it again does nothing: it wasn't a
// peek, so it isn't z's to close.
export function peekBeside(s: ChromeState, to: string, here?: string): ChromeState {
  if (!s.beside.tabs.includes(to)) {
    const opened = openBeside(s, to, { inPlaceOf: s.beside.peek, here });
    return { ...opened, beside: { ...opened.beside, peek: to } };
  }
  if (besideShown(s, here) && besideFront(s, here) === to) return s.beside.peek === to ? closeBesideTab(s, to, here) : s;
  // One that's about another thing's page is asked for here: it's there
  // anywhere from now.
  const anywhere = besideTabs(s, here).all.includes(to) ? s : { ...s, beside: { ...s.beside, about: without(s.beside.about, to) } };
  const picked = pickBeside(anywhere, to, here);
  return { ...picked, beside: { ...picked.beside, open: true } };
}

export const toggleBeside = (s: ChromeState): ChromeState => ({ ...s, beside: { ...s.beside, open: !s.beside.open } });

export const floatBeside = (s: ChromeState, over: boolean): ChromeState => ({ ...s, beside: { ...s.beside, over } });

const widthWithin = (w: unknown): number => (typeof w === "number" && Number.isFinite(w) ? Math.round(Math.min(besideWidths.max, Math.max(besideWidths.min, w))) : besideWidths.rest);

// widenBeside sets Beside's width, kept within what it may be.
export const widenBeside = (s: ChromeState, width: number): ChromeState => ({ ...s, beside: { ...s.beside, width: widthWithin(width) } });

export const toggleBelow = (s: ChromeState): ChromeState => ({ ...s, below: { ...s.below, open: !s.below.open } });

// pickBelow shows one of Below's views, where Below is: open or not.
export const pickBelow = (s: ChromeState, tab: string): ChromeState => (s.below.tab === tab ? s : { ...s, below: { ...s.below, tab } });

// showBelow opens Below on one of its views, as something that points at
// it does (a card's "Output below", a part of the foot). Pressed while
// that view is the one showing, it closes Below: what opens it is its
// toggle.
export function showBelow(s: ChromeState, tab: string): ChromeState {
  const showing = s.below.open && s.below.tab === tab;
  return { ...s, below: { ...s.below, open: !showing, tab } };
}

// seenBelow is Below's having been looked at, at: what started before is
// no longer new.
export const seenBelow = (s: ChromeState, at: number): ChromeState => ({ ...s, below: { ...s.below, seen: at } });

export const belowWhere = (s: ChromeState, where: BelowWhere): ChromeState => (s.below.where === where ? s : { ...s, below: { ...s.below, where } });

const heightWithin = (h: unknown): number => (typeof h === "number" && Number.isFinite(h) ? Math.round(Math.min(belowHeights.max, Math.max(belowHeights.min, h))) : belowHeights.rest);

// heightenBelow sets Below's height, kept within what it may be.
export const heightenBelow = (s: ChromeState, height: number): ChromeState => ({ ...s, below: { ...s.below, height: heightWithin(height) } });

// strings is what's kept of a map of words to words: those of it that are.
function strings(kept: unknown): Record<string, string> {
  if (!kept || typeof kept !== "object" || Array.isArray(kept)) return {};
  return Object.fromEntries(Object.entries(kept).filter((e): e is [string, string] => typeof e[1] === "string" && e[1] !== ""));
}

// read is the arrangement as it was kept, with anything that's missing or
// isn't what it should be (kept by an earlier ws, or edited) as it rests.
export function read(kept: string | null | undefined): ChromeState {
  let got: { hidden?: unknown; beside?: Record<string, unknown>; below?: Record<string, unknown> } = {};
  try {
    const parsed: unknown = JSON.parse(kept ?? "");
    if (parsed && typeof parsed === "object") got = parsed;
  } catch {
    // Nothing kept, or not what was kept: as it rests.
  }
  const beside = got.beside && typeof got.beside === "object" ? got.beside : {};
  const below = got.below && typeof got.below === "object" ? got.below : {};
  // One that hadn't begun (/ask/…) was its page's, which isn't open yet.
  const tabs = Array.isArray(beside.tabs) ? [...new Set(beside.tabs.filter((t): t is string => typeof t === "string" && t !== "" && !t.startsWith("/ask/")))] : [];
  // What a tab's about is kept only for tabs there are.
  const about = Object.fromEntries(Object.entries(strings(beside.about)).filter(([t]) => tabs.includes(t)));
  return {
    hidden: got.hidden === true,
    beside: { open: beside.open === true, over: beside.over === true, width: widthWithin(beside.width), tabs, about, front: typeof beside.front === "string" ? beside.front : undefined, fronts: strings(beside.fronts), peek: typeof beside.peek === "string" && tabs.includes(beside.peek) ? beside.peek : undefined },
    below: { open: below.open === true, height: heightWithin(below.height), tab: typeof below.tab === "string" && below.tab !== "" ? below.tab : undefined, where: below.where === "everywhere" ? "everywhere" : "here", seen: typeof below.seen === "number" && Number.isFinite(below.seen) ? below.seen : undefined },
  };
}

export const write = (s: ChromeState): string => JSON.stringify(s);
