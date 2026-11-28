import { setPref, usePrefs } from "../prefs";
import { type Command, isCommand, type Scope } from "./commands";

// The keymap is which keys do each command: its own (what its scope says
// it has), or the ones you've set in their place (Settings › Keyboard),
// kept in this browser with the rest of your prefs (prefs.ts, keymap).

type Map = Record<string, string[]>;
const mine = (): Map => usePrefs.getState().keymap;

// keysOf is the keys that do c now, the one that's said first.
export function keysOf(c: Command, map: Map = mine()): readonly string[] {
  if (c.fixed) return c.defaults;
  return Object.hasOwn(map, c.id) ? map[c.id] : c.defaults;
}

// keyOf is c's key as it's said: the first of its keys, if it has one.
export const keyOf = (c: Command, map: Map = mine()): string | undefined => keysOf(c, map)[0];

// saidOf is a key as a part is told it: a command's key, or a key
// written out ("esc", for a key the part's page binds itself).
export const saidOf = (k: string | Command | undefined, map: Map = mine()): string | undefined => (isCommand(k) ? keyOf(k, map) : k);

// useKeymap is what you've set, for a part that says keys to be drawn
// again when it changes.
export const useKeymap = (): Map => usePrefs((s) => s.keymap);

// useSaid is saidOf, drawn again when the keymap changes.
export function useSaid(k: string | Command | undefined): string | undefined {
  return saidOf(k, useKeymap());
}

// isOwn is whether c has its own keys (you haven't set others).
export const isOwn = (c: Command, map: Map = mine()): boolean => !Object.hasOwn(map, c.id);

// setKeys gives c keys in place of its own; its own again takes it back
// to them, as if never set.
export function setKeys(c: Command, keys: readonly string[]) {
  if (c.fixed) return;
  setPref("keymap", (was) => {
    const { [c.id]: _, ...rest } = was;
    return same(keys, c.defaults) ? rest : { ...rest, [c.id]: [...keys] };
  });
}

// takeSuggested gives c the keys it was suggested, beside any it has.
export function takeSuggested(c: Command) {
  const had = keysOf(c);
  setKeys(c, [...had, ...c.suggested.filter((k) => !had.includes(k))]);
}

// resetKeys takes c, or every command, back to its own keys.
export function resetKeys(c?: Command) {
  setPref("keymap", (was) => {
    if (!c) return {};
    const { [c.id]: _, ...rest } = was;
    return rest;
  });
}

const same = (a: readonly string[], b: readonly string[]) => a.length === b.length && a.every((k, i) => k === b[i]);

// A clash is a key that does two commands where only one can be done: two
// of one scope, or one of a scope and one of everywhere's (which is read
// first, so the scope's is never done), or one then another that starts
// with another's key. Only keys you've set clash: two
// with the same key of their own are meant to (j is the next hit while
// searching, the next row otherwise, and only one is bound at a time).
export interface Clash {
  key: string;
  with: Command;
  // It's one then another that starts with the other's key (j then j,
  // where j is Next): pressed twice quickly, the other's isn't done twice.
  starts?: string;
}

// clashesOf is c's clashes: with its scope's commands, and with
// everywhere's (global), and with also's: for a plugin's action, which is
// bound on every page it's for, every other scope's.
export function clashesOf(c: Command, global: Scope | undefined, map: Map = mine(), also: readonly Scope[] = []): Clash[] {
  if (c.fixed) return [];
  const out: Clash[] = [];
  const mineKeys = keysOf(c, map);
  const scopes = [c.scope, ...(global && global !== c.scope ? [global] : []), ...also.filter((s) => s !== c.scope && s !== global)];
  const others = scopes.flatMap((s) => s.commands);
  for (const o of others) {
    if (o === c || o.fixed) continue;
    if (isOwn(c, map) && isOwn(o, map)) continue;
    const theirs = keysOf(o, map);
    for (const k of mineKeys) {
      if (theirs.includes(k)) out.push({ key: k, with: o });
      const first = k.split(" ")[0];
      if (first !== k && theirs.includes(first)) out.push({ key: k, with: o, starts: first });
      for (const t of theirs) if (t !== k && t.split(" ")[0] === k && t.includes(" ")) out.push({ key: t, with: o, starts: k });
    }
  }
  return out;
}

// KeyLike is a key as a part is told it: a command (whose key is whichever
// it is now), or a key written out, for one the page binds itself.
export type KeyLike = string | Command;
