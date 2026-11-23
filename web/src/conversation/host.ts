import { useSyncExternalStore } from "react";

// A page's conversation: one conversation about what a page is of. The
// ask field at the page's foot is the way in; asking opens the
// conversation beside the page (Beside, chrome/), where it's held from
// then. Who decides what:
//
// - The page says what may be had (set): the bar or not, what the
//   person's holding, what the agent should know. It can open the
//   conversation too, with a message, and close it.
// - The person has it beside the page, docked or floating as Beside is,
//   which is the window's to keep and not the page's.
//
// The host is the one implementation. ws's own pages call it directly;
// a plugin's page says the same over the frame protocol (ws.conversation
// in @withworkspace/ui-lib), and the frame's host (plugins/frame/v1.ts)
// passes each message on to it as it is.
//
// The protocol's forms are from when the page's conversation was drawn
// three ways of its own (a card over the bar, docked, a popover). A page
// may still name them, and the protocol is as it was: any of them is
// Beside, and where the conversation is, is said back as docked or
// popover, as Beside is next to the page or over it.
export type Form = "card" | "docked" | "popover";

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
  // opening is the page's latest asking, until the view takes it.
  opening?: Opening;
  // closing is a count of the times the page has asked for it closed.
  closing: number;
  // focus is a count of the times the bar's been asked to take focus.
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
  // took is the view having taken the page's opening.
  took(n: number): void;
  // focusBar asks the view to go to the bar.
  focusBar(): void;
  // shown is the view saying where it is, which the page hears.
  shown(status: Status): void;
  // forget is the page gone: what it said goes, and it closes.
  forget(): void;
}
// createHost makes a page's conversation host.
export function createHost(): ConversationHost {
  let state: State = { settings: unsaid, closing: 0, focus: 0 };
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
      update({ opening: { ...o, n: ++openings } });
    },
    close() {
      update({ opening: undefined, closing: state.closing + 1 });
    },
    onChange(fn) {
      changed.add(fn);
      return () => void changed.delete(fn);
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
      update({ settings: unsaid, opening: undefined, closing: state.closing + 1 });
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
