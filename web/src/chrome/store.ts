import type { V1 } from "@withworkspace/ui-lib/protocol";
import { useEffect, useMemo, useRef } from "react";
import { create } from "zustand";
import type { TalkDetail, TalkGiven, TalkProposal } from "../api";
import { besideAbout, besideFront, besideTabs, type ChromeState, closeBesideTab, hideSidebar, openBeside, peekBeside, read, write } from "./chromeState";
import { paneAt } from "./panes";

// The window's state, in one store: how it's arranged (chromeState.ts:
// the sidebar, and what's beside the page), what the page that's open is
// of, and what that page gives the things beside it. Anything in the
// window reads the part it needs and is drawn again only when that part
// changes; anything can change it, from wherever it is: the ask bar deep
// in a page opens a conversation beside without the page between them
// knowing.
//
// How it's arranged is kept in the browser for the workspace, and is what
// the window opens with next time. The rest is this page load's: what's
// open over the window (Find, the keys, the sidebar as a drawer, the
// sidebar being arranged), whether the daemon's been shut down from it,
// and what a plugin's page in the frame has said of itself.

// FrameHere is what a plugin's page says it shows beyond its address
// (ws:here): an issue open in a panel over the board, by one of ws's
// names. Find starts from it on that plugin's pages, over what the
// address says.
export interface FrameHere {
  plugin: string;
  name: string;
}

// PageGives is what the open page gives a conversation beside it: what
// only the page has. A conversation beside is a tab like any other;
// while the page it was asked on is open, that page gives it what you're
// holding there, the page's state as the agent should know it, and where
// an answer's citation leads. With the page gone, it carries on without
// them.
export interface PageGives {
  // What goes to the agent with what's said: the page's state, and what
  // you're holding there.
  given?: () => TalkGiven | undefined;
  // What you're holding on the page, as a chip on the reply.
  holding?: { label: string; title?: string };
  // Going to what an answer cites on the page (#how-it-lands): false if
  // the page hasn't got it.
  cite?: (href: string) => boolean;
  // Something it proposed was accepted or declined beside the page.
  onSettled?: (p: TalkProposal) => void;
  // For one that hasn't begun (askAt): what it's about, what to offer
  // asking, and how asking begins it.
  thing?: string;
  placeholder?: string;
  suggestions?: string[];
  start?: (body: string) => Promise<TalkDetail>;
}

const key = (root: string) => `ws:chrome:${root}`;
// Which workspace this browser last arranged here: its arrangement is
// what the page opens with, before the workspace has said which it is.
const lastKey = "ws:chrome:last";

function kept(root: string): ChromeState {
  try {
    const of = root || localStorage.getItem(lastKey) || "";
    if (root) localStorage.setItem(lastKey, root);
    return read(of ? localStorage.getItem(key(of)) : undefined);
  } catch {
    return read(undefined);
  }
}

interface ChromeStore {
  // The workspace this is kept for ("" until it's said which it is).
  root: string;
  chrome: ChromeState;
  // What the page that's open is of, as Find's Here has it, by id
  // (mission:dark-mode, document:records:adr/3.md, thing:jira/CR-12). A
  // tab opened about that thing is there only while this says the same.
  here?: string;
  // What the open page gives each tab, by the tab's address.
  gives: Record<string, PageGives>;
  // How many times each tab's been asked to take the keys, until it has.
  keys: Record<string, number>;

  // Find, open: with what it starts with typed ("/" for files, from ⌘P).
  // Undefined is closed.
  find?: string;
  // The keys' list is open.
  help: boolean;
  // On a narrow window the sidebar's a drawer: open or not.
  drawer: boolean;
  // bell is the foot's bell's list of what you haven't read: open or not.
  bell: boolean;
  // The sidebar's being arranged: its places, and which are in the menu.
  arranging: boolean;
  // The daemon was shut down from this window: the page went with it.
  closed: boolean;
  // What a plugin's page in the frame says (plugins/PluginFrame.tsx), for the
  // shell, far from it in the tree. dim: its app has a backdrop up, which
  // can only dim the frame, so the shell dims what's beside the frame in
  // the same scrim, and the two read as one. here: what it shows beyond
  // its address. expanded: the plugin whose page asked for ws's chrome
  // expanded (ws:chrome-layout): the shell folds its sidebar to a rail
  // while ws is on that plugin's pages. Each is forgotten when the page
  // goes.
  frame: { dim: boolean; here?: FrameHere; expanded?: string };

  setRoot: (root: string | undefined) => void;
  // A change to how it's arranged, as a function of it (chromeState.ts's).
  change: (by: (s: ChromeState) => ChromeState) => void;
  setHere: (id: string | undefined) => void;
  // give is the page giving a tab what it has, until what's returned is
  // called (the page has gone, or gives something else).
  give: (to: string, g: PageGives) => () => void;
  // open opens a thing beside, in front, by its address. here: it's about
  // the thing this page is of, and so is there only on that thing's page.
  // inPlaceOf: it takes that tab's place, if that's there. keys: its pane
  // takes the keys (a conversation's reply).
  open: (to: string, o?: { here?: boolean; inPlaceOf?: string; keys?: boolean }) => void;
  close: (to: string) => void;
  // peek is z on a thing, by its address: it opens beside as the peek, in
  // the last peek's place; on the peek in front, it closes it.
  peek: (to: string) => void;
  // takeKeys is a pane taking the keys it was asked to: true once for
  // each asking, whenever the pane comes to it.
  takeKeys: (to: string) => boolean;

  // Find, opened (with what it starts with typed) and closed; the keys'
  // list, which takes Find's place if that's open.
  openFind: (start?: string) => void;
  closeFind: () => void;
  setHelp: (open: boolean) => void;
  setDrawer: (open: boolean | ((was: boolean) => boolean)) => void;
  setBell: (open: boolean | ((was: boolean) => boolean)) => void;
  // arrange starts arranging the sidebar, where it is: shown, and on a
  // narrow window, the drawer open. stopArranging ends it, unsaved.
  arrange: () => void;
  stopArranging: () => void;
  daemonClosed: () => void;
  // What the frame's page says. None (here), or standard (layout), from
  // a plugin whose page isn't the one that said the last leaves that
  // one's: a page going as another comes doesn't undo the new one's.
  setFrameDim: (on: boolean) => void;
  setFrameHere: (plugin: string, name: string | undefined) => void;
  setFrameLayout: (plugin: string, mode: V1.ChromeMode) => void;
}

export const useChromeStore = create<ChromeStore>()((set, get) => ({
  root: "",
  chrome: kept(""),
  here: undefined,
  gives: {},
  keys: {},
  find: undefined,
  help: false,
  drawer: false,
  bell: false,
  arranging: false,
  closed: false,
  frame: { dim: false },

  setRoot: (root) => {
    const of = root ?? "";
    if (get().root === of) return;
    set({ root: of, chrome: kept(of) });
  },
  change: (by) => {
    const { root, chrome } = get();
    const next = by(chrome);
    if (next === chrome) return;
    set({ chrome: next });
    try {
      if (root) localStorage.setItem(key(root), write(next));
    } catch {
      // Nowhere to keep it: it holds while the page is open.
    }
  },
  setHere: (id) => {
    if (get().here !== id) set({ here: id });
  },
  give: (to, g) => {
    set((s) => ({ gives: { ...s.gives, [to]: g } }));
    return () => {
      if (get().gives[to] !== g) return;
      set((s) => {
        const { [to]: _, ...rest } = s.gives;
        return { gives: rest };
      });
    };
  },
  open: (to, o = {}) => {
    const { here, change } = get();
    change((c) => openBeside(c, to, { inPlaceOf: o.inPlaceOf, here, about: o.here ? here : undefined }));
    if (o.keys) set((s) => ({ keys: { ...s.keys, [to]: (s.keys[to] ?? 0) + 1 } }));
  },
  close: (to) => {
    const { here, change } = get();
    change((c) => closeBesideTab(c, to, here));
    set((s) => {
      if (!(to in s.keys)) return s;
      const { [to]: _, ...rest } = s.keys;
      return { keys: rest };
    });
  },
  peek: (to) => {
    const { here, change } = get();
    change((c) => peekBeside(c, to, here));
  },
  takeKeys: (to) => {
    if (!get().keys[to]) return false;
    set((s) => {
      const { [to]: _, ...rest } = s.keys;
      return { keys: rest };
    });
    return true;
  },

  openFind: (start = "") => set({ find: start }),
  closeFind: () => {
    if (get().find !== undefined) set({ find: undefined });
  },
  setHelp: (open) => set(open ? { help: true, find: undefined } : { help: false }),
  setDrawer: (open) => {
    const to = typeof open === "function" ? open(get().drawer) : open;
    if (to !== get().drawer) set({ drawer: to });
  },
  setBell: (open) => {
    const to = typeof open === "function" ? open(get().bell) : open;
    if (to !== get().bell) set({ bell: to });
  },
  arrange: () => {
    get().change((c) => hideSidebar(c, false));
    set({ arranging: true, drawer: true });
  },
  stopArranging: () => {
    if (get().arranging) set({ arranging: false });
  },
  daemonClosed: () => set({ closed: true }),
  setFrameDim: (on) => {
    if (on !== get().frame.dim) set((s) => ({ frame: { ...s.frame, dim: on } }));
  },
  setFrameHere: (plugin, name) => {
    const was = get().frame.here;
    const next = name ? { plugin, name } : was?.plugin === plugin ? undefined : was;
    if (next === was || (next && was && next.plugin === was.plugin && next.name === was.name)) return;
    set((s) => ({ frame: { ...s.frame, here: next } }));
  },
  setFrameLayout: (plugin, mode) => {
    const was = get().frame.expanded;
    const next = mode === "expanded" ? plugin : was === plugin ? undefined : was;
    if (next !== was) set((s) => ({ frame: { ...s.frame, expanded: next } }));
  },
}));

// What the frame's page said, for the shell: whether to dim what's beside
// the frame, what the page shows, and the plugin whose page asked for the
// chrome expanded. (They're drawn in the turn they change, as anything
// read from the store is: the dim comes and goes with the app's
// backdrop.)
export const useFrameDim = (): boolean => useChromeStore((s) => s.frame.dim);
export const useFrameHere = (): FrameHere | undefined => useChromeStore((s) => s.frame.here);
export const useFrameExpanded = (): string | undefined => useChromeStore((s) => s.frame.expanded);

// drawable is whether a tab has a pane to draw: its address has one, and
// for a conversation that hasn't begun, the page it's about is the one
// open.
const drawable = (t: string, gives: Record<string, PageGives>) => {
  const at = paneAt(t);
  return !!at && (at.kind !== "ask" || !!gives[t]?.start);
};

// What's beside the page that's open, as it's drawn.
export interface BesideNow {
  // The tabs about this page's thing, then those that are there anywhere:
  // only those with a pane to draw.
  here: string[];
  anywhere: string[];
  all: string[];
  // The one in front.
  front?: string;
  // Beside is drawn: it's open, with something in it here.
  shown: boolean;
  // Over the page, not next to it.
  over: boolean;
  width: number;
  // Every tab about this page's thing, drawable or not, in the order
  // opened: what was asked here before, as the page finds it again.
  about: string[];
}

export function besideNow(chrome: ChromeState, here: string | undefined, gives: Record<string, PageGives>): BesideNow {
  const tabs = besideTabs(chrome, here);
  const mine = tabs.here.filter((t) => drawable(t, gives));
  const anywhere = tabs.anywhere.filter((t) => drawable(t, gives));
  const all = [...mine, ...anywhere];
  const front = [besideFront(chrome, here), all[0]].find((t) => t !== undefined && all.includes(t));
  return {
    here: mine,
    anywhere,
    all,
    front,
    shown: chrome.beside.open && all.length > 0,
    over: chrome.beside.over,
    width: chrome.beside.width,
    about: chrome.beside.tabs.filter((t) => here !== undefined && besideAbout(chrome, t) === here),
  };
}

// useBeside is what's beside the page that's open: for Beside itself, the
// foot's toggle, and a page that would know whether what it opened is
// still there.
export function useBeside(): BesideNow {
  const chrome = useChromeStore((s) => s.chrome);
  const here = useChromeStore((s) => s.here);
  const gives = useChromeStore((s) => s.gives);
  return useMemo(() => besideNow(chrome, here, gives), [chrome, here, gives]);
}

// useGives is what the page gives a tab, as its pane draws it.
export const useGives = (to: string): PageGives | undefined => useChromeStore((s) => s.gives[to]);

// useKeysAsked changes each time a tab's asked to take the keys.
export const useKeysAsked = (to: string): number => useChromeStore((s) => s.keys[to] ?? 0);

// useGive is a page giving a tab what it has, for as long as the page is
// there and the tab is that one. What it calls is always the page's
// latest; what it shows (the chip, what's offered) is given again when it
// changes.
export function useGive(to: string | undefined, g: PageGives) {
  const latest = useRef(g);
  latest.current = g;
  const shows = JSON.stringify([g.holding, g.thing, g.placeholder, g.suggestions, !!g.cite, !!g.start]);
  useEffect(() => {
    if (!to) return;
    const now = latest.current;
    return useChromeStore.getState().give(to, {
      holding: now.holding,
      thing: now.thing,
      placeholder: now.placeholder,
      suggestions: now.suggestions,
      given: () => latest.current.given?.(),
      cite: now.cite && ((href) => latest.current.cite?.(href) ?? false),
      onSettled: (p) => latest.current.onSettled?.(p),
      start: now.start && ((body) => latest.current.start?.(body) ?? Promise.reject(new Error("The page it was asked on has gone."))),
    });
  }, [to, shows]);
}
