import { keyName, leaderWithin, typing } from "../keys";
import type { Command, Scope } from "./commands";
import { keysOf } from "./keymap";

// One listener does every command a key does (useCommands binds them).
// A press is named (keyName), and the commands bound now are read for
// one with that key: the first bound that has it is done, and that's
// the press's.
//
// Which is first:
// - Everywhere's keys that are read before anything (bound first: ⌘K,
//   g then a letter) are read as the key's pressed, before what has the
//   focus sees it (the window's capture).
// - The rest after it has, and only if it left the key alone: a menu's
//   arrows are the menu's, an editor's Escape the editor's.
// - Among those, the one bound earlier: a part's before the page it's in,
//   as each was when it had a listener of its own.
// - While a modal (the Keys dialog, a finder, the picture viewer) is
//   bound, only a modal's are done, with those read first.
//
// A key typed into a field is the field's, apart from Escape and the
// commands that say they're done in fields too.
//
// A key then another ("g m") is done when the second is pressed soon
// after the first (leaderWithin). The first is still done by itself, if
// it does something by itself: on a list, g is the first row, and g then
// m is Missions. One then another is read before the second by itself.

// pass is what a handler gives back when the key isn't its, now: it's
// left to the next, and to the browser (⌘K in an editor, with words
// selected, is a link). Anything else it gives back (false from a && b())
// is the key taken, as it was.
export const pass: unique symbol = Symbol("pass");

// A command's handler.
export type Handler = (e: KeyboardEvent) => unknown;

export interface Bound {
  scope: Scope;
  // What's bound now, by command name: read each press, so the latest
  // handlers are the ones done.
  handlers: () => Partial<Record<string, Handler | false | undefined>>;
  modal: boolean;
  first: boolean;
  // Read after the rest (a plugin's actions): a page's own key is the
  // page's, whatever was bound first.
  late?: boolean;
}

const bound: Bound[] = [];
let listeners: (() => void) | undefined;

// bind adds what's bound, until what it gives back is called.
export function bind(b: Bound): () => void {
  bound.push(b);
  listen();
  return () => {
    const i = bound.indexOf(b);
    if (i >= 0) bound.splice(i, 1);
  };
}

// The key pressed before this one, if it starts one then another.
let pending: { key: string; at: number } | undefined;

// What's read of a press, once: in the capture, then again after the
// focus has had it.
const reads = new WeakMap<KeyboardEvent, { name: string; seq?: string; field: boolean }>();

const plain = new Set(["Shift", "Control", "Alt", "Meta", "CapsLock", "Dead", "Unidentified"]);

function read(e: KeyboardEvent) {
  let r = reads.get(e);
  if (r) return r;
  const name = keyName(e);
  const now = Date.now();
  const seq = pending && now - pending.at < leaderWithin ? `${pending.key} ${name}` : undefined;
  const field = typing(e.target);
  // A key that begins one then another waits for the next: unless it's
  // typed, or held with ⌘. (One that ends one then another doesn't, once
  // that's done: press, below.)
  pending = !field && !e.metaKey && !e.ctrlKey && begins(name) ? { key: name, at: now } : undefined;
  r = { name, seq, field };
  reads.set(e, r);
  return r;
}

// begins is whether a key is the first of one then another that's bound.
function begins(name: string): boolean {
  const start = `${name} `;
  return bound.some((b) => {
    const hs = b.handlers();
    return b.scope.commands.some((c) => hs[c.name] && keysOf(c).some((k) => k.startsWith(start)));
  });
}

// asked is who's asked about a press, in order.
function asked(first: boolean): Bound[] {
  const modal = bound.some((b) => b.modal);
  // Everywhere's that's read first is read whatever's open: it says
  // itself when it isn't its key (Find's open, say).
  if (first) return bound.filter((b) => b.first);
  const rest = bound.filter((b) => !b.first);
  const ordered = [...rest.filter((b) => !b.late), ...rest.filter((b) => b.late)];
  return modal ? ordered.filter((b) => b.modal) : ordered;
}

// done does the first command bound to key among bs, and says whether one took it.
function done(bs: Bound[], key: string, e: KeyboardEvent, field: boolean, did: (c: Command) => void): boolean {
  for (const b of bs) {
    const hs = b.handlers();
    for (const c of b.scope.commands) {
      const h = hs[c.name];
      if (!h || c.fixed) continue;
      if (field && !c.fields && key !== "Escape") continue;
      if (!keysOf(c).includes(key)) continue;
      if (h(e) === pass) continue;
      e.preventDefault();
      did(c);
      return true;
    }
  }
  return false;
}

// While a key's being set (Settings › Keyboard), every key pressed is
// its: none does what it does.
let held: ((e: KeyboardEvent) => void) | undefined;

// holdKeys gives every key pressed to fn, and only to it, until what it
// gives back is called.
export function holdKeys(fn: (e: KeyboardEvent) => void): () => void {
  held = fn;
  pending = undefined;
  listen();
  return () => {
    if (held === fn) held = undefined;
  };
}

// press is a key pressed, in the capture (first) or after the focus has
// had it.
export function press(e: KeyboardEvent, first: boolean): Command | undefined {
  if (held) {
    if (!first) return undefined;
    e.preventDefault();
    e.stopPropagation();
    if (!e.isComposing && !plain.has(e.key)) held(e);
    return undefined;
  }
  if (e.isComposing || plain.has(e.key)) return undefined;
  const r = read(e);
  if (!first && e.defaultPrevented) return undefined;
  const bs = asked(first);
  let got: Command | undefined;
  const did = (c: Command) => (got = c);
  if (r.seq && done(bs, r.seq, e, r.field, did)) pending = undefined;
  else done(bs, r.name, e, r.field, did);
  if (got?.moves && typeof document !== "undefined") document.body.classList.add("keyboarding");
  return got;
}

function listen() {
  if (listeners || typeof window === "undefined") return;
  const capture = (e: KeyboardEvent) => {
    if (press(e, true)) e.stopPropagation();
  };
  const after = (e: KeyboardEvent) => void press(e, false);
  window.addEventListener("keydown", capture, true);
  window.addEventListener("keydown", after);
  listeners = () => {
    window.removeEventListener("keydown", capture, true);
    window.removeEventListener("keydown", after);
  };
}

// boundNow is what's bound now, in order: for the Keys dialog to say
// what this page's keys are, and the frame its pass-ons.
export function boundNow(): readonly Bound[] {
  return bound;
}

// isBound is whether c is bound now.
export function isBound(c: Command): boolean {
  return bound.some((b) => b.scope === c.scope && !!b.handlers()[c.name]);
}

// forgetAll is for tests: nothing bound, no key waiting for another.
export function forgetAll() {
  bound.length = 0;
  pending = undefined;
  held = undefined;
}
