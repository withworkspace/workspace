import { useSyncExternalStore } from "react";

// A page's conversation: one conversation about what a page is of, drawn
// by ws in the page's chrome. The ask bar at the page's foot is the
// passive way in; asking opens the answer as a card over it; the card pops
// out to a pane docked beside the page, or a popover over its corner, and
// folds back. Who decides what:
//
// - The page says what may be had (set): the bar or not, the forms it
//   allows, what the person's holding, what the agent should know. It can
//   open the conversation too, with a message, and close it.
// - The person moves it between the forms, and ws remembers their choice
//   for the page's owner (a plugin): it opens that way next time.
// - ws fits it to the window: too narrow to dock, it floats; a page that
//   allows nothing that fits gets the card.
//
// The host is the one implementation. ws's own pages call it directly;
// a plugin's page says the same over the frame protocol (ws.conversation
// in @withworkspace/ui-lib), and the frame's host (plugins/frame/v1.ts)
// passes each message on to it as it is.

export type Form = "card" | "docked" | "popover";
export const FORMS: readonly Form[] = ["card", "docked", "popover"];
export const isForm = (v: unknown): v is Form => FORMS.includes(v as Form);

// Holding is what the person has in hand on the page: a selection. It's a
// chip on the bar, and given to the agent with the next thing asked.
export interface Holding {
  label: string;
  names?: string[];
  text?: string;
}

// Settings is all a page says of its conversation, each time: it takes
// the place of what it said before.
export interface Settings {
  ask: boolean;
  allow: Form[];
  placeholder?: string;
  holding?: Holding;
  // context is the page's state as words, for the agent: given with a
  // message only when it's changed since the agent last had it (the
  // server diffs it: conversation.Give).
  context?: string;
  suggestions?: string[];
}

// Nothing said is none: no bar, and no forms.
export const unsaid: Settings = { ask: false, allow: [] };

// Status is where the conversation is, as a page hears it: open or not,
// the form it's shown in, and whether its agent's working.
export interface Status {
  open: boolean;
  as?: Form;
  working: boolean;
}

// Opening is a page asking for the conversation to open: with a message
// said as the person's, and context given with it. n tells one asking
// from the next, so the same message asked twice is asked twice.
export interface Opening {
  n: number;
  message?: string;
  context?: string;
  as?: Form;
}

// State is the host as its view draws it.
export interface State {
  settings: Settings;
  open: boolean;
  // want is the form asked for: by the page, the person's last move, or
  // what they chose before. The view fits it to what's allowed and the
  // window.
  want: Form;
  // opening is the page's latest asking, until the view takes it.
  opening?: Opening;
  // focus is a count of the times the bar's been asked to take focus (/,
  // ⌘J): the view focuses it each time it moves.
  focus: number;
}

export interface ConversationHost {
  // What the page says: set, open, close, onChange.
  set(settings: Settings): void;
  open(o?: Omit<Opening, "n">): void;
  close(): void;
  onChange(fn: (status: Status) => void): () => void;
  // What ws's view of it does.
  readonly state: State;
  subscribe(fn: () => void): () => void;
  // move is the person moving it: to a form (remembered), or closing it.
  move(to: Form | undefined): void;
  // opened is the view having it open in a form (a send opened it, say).
  opened(as?: Form): void;
  // took is the view having taken the page's opening.
  took(n: number): void;
  // focusBar asks the view to go to the bar (/, ⌘J).
  focusBar(): void;
  // shown is the view saying where it is, which the page hears.
  shown(status: Status): void;
  // forget is the page gone: what it said goes, and it closes.
  forget(): void;
}

// Where the person's choice of form is kept, for each owner.
const prefKey = (owner: string) => `ws:conversation-form:${owner}`;

export function preferred(owner: string): Form | undefined {
  try {
    const v = localStorage.getItem(prefKey(owner));
    return isForm(v) ? v : undefined;
  } catch {
    return undefined;
  }
}

function prefer(owner: string, form: Form) {
  try {
    localStorage.setItem(prefKey(owner), form);
  } catch {
    // Kept for this page only, then.
  }
}

// fit is the form a conversation's shown in: the one wanted, if the page
// allows it and the window has room; docked becomes the popover where
// it's narrow; else the first allowed that fits, and the card when
// nothing does.
export function fit(want: Form, allow: readonly Form[], narrow: boolean): Form {
  let as = want === "docked" && narrow ? "popover" : want;
  if (!allow.includes(as)) as = allow.find((f) => !(f === "docked" && narrow)) ?? "card";
  return as;
}

// createHost makes a page's conversation host, for an owner (a plugin's
// name) whose person's choice of form it remembers.
export function createHost(owner: string): ConversationHost {
  let state: State = { settings: unsaid, open: false, want: preferred(owner) ?? "card", focus: 0 };
  let status: Status = { open: false, working: false };
  let openings = 0;
  const heard = new Set<() => void>();
  const changed = new Set<(s: Status) => void>();
  const update = (next: Partial<State>) => {
    state = { ...state, ...next };
    for (const fn of [...heard]) fn();
  };
  return {
    get state() {
      return state;
    },
    subscribe(fn) {
      heard.add(fn);
      return () => void heard.delete(fn);
    },
    set(settings) {
      if (JSON.stringify(settings) === JSON.stringify(state.settings)) return;
      update({ settings });
    },
    open(o = {}) {
      const want = o.as ?? preferred(owner) ?? state.want;
      update({ open: true, want, opening: { ...o, n: ++openings } });
    },
    close() {
      update({ open: false });
    },
    onChange(fn) {
      changed.add(fn);
      return () => void changed.delete(fn);
    },
    move(to) {
      if (!to) return update({ open: false });
      prefer(owner, to);
      update({ open: true, want: to });
    },
    opened(as) {
      if (state.open && (!as || as === state.want)) return;
      update({ open: true, ...(as ? { want: as } : {}) });
    },
    took(n) {
      if (state.opening?.n === n) update({ opening: undefined });
    },
    focusBar() {
      update({ focus: state.focus + 1 });
    },
    shown(s) {
      if (s.open === status.open && s.as === status.as && s.working === status.working) return;
      status = s;
      for (const fn of [...changed]) fn(s);
    },
    forget() {
      update({ settings: unsaid, open: false, opening: undefined });
    },
  };
}

// useHost is the host's state, for its view.
export function useHost(host: ConversationHost): State {
  return useSyncExternalStore(host.subscribe, () => host.state);
}

// givenWith is what the page gives the agent with a message: its standing
// context (which the server gives only if it's changed since the agent
// last had it), what's held, and the message's own context. undefined
// when there's nothing.
export function givenWith(from: string, s: Settings, context?: string) {
  const g = {
    from,
    ...(s.context?.trim() ? { page: s.context } : {}),
    ...(s.holding ? { holding: s.holding.label, ...(s.holding.text?.trim() ? { holdingText: s.holding.text } : {}) } : {}),
    ...(context?.trim() ? { context } : {}),
  };
  return Object.keys(g).length > 1 ? g : undefined;
}
