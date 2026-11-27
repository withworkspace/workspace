import type { Command, Scope } from "./commands";
import { boundNow } from "./dispatch";
import { keyOf, keysOf } from "./keymap";

// What the Keys dialog says: the keys of what's open, a group for each
// scope that's bound (the page's, then those of the parts in it), then
// everywhere's. Each says the commands bound now, so a key that's only
// a key while searching is said while searching, and what its scope does
// with keys of its own (fixed: an editor's ```).

export interface HelpGroup {
  title: string;
  keys: [string, string][];
}

// rowsOf is commands as the dialog's lines: a key and what it does. Those
// that share a row are one line, their first keys side by side ("j / k",
// "Next / previous"); one alone says all its keys ("⏎ / o", "Open"). One
// with no key now isn't said.
export function rowsOf(commands: readonly Command[], map?: Record<string, string[]>): [string, string][] {
  const rows: { what: string; keys: string[] }[] = [];
  const byRow = new Map<string, { what: string; keys: string[] }>();
  for (const c of commands) {
    // Alone, a command says each of its keys (⏎ / o); in a row with
    // others, its first (j / k).
    const k = c.row ? keyOf(c, map) : keysOf(c, map).join(" / ");
    if (!k) continue;
    const name = c.row ?? c.title;
    const had = c.row ? byRow.get(name) : undefined;
    if (had) {
      if (!had.keys.includes(k)) had.keys.push(k);
      continue;
    }
    const row = { what: name, keys: [k] };
    rows.push(row);
    if (c.row) byRow.set(name, row);
  }
  return rows.map((r) => [run(r.keys) ?? r.keys.join(" / "), r.what]);
}

// helpGroups is what the dialog says now: everywhere is the scope said
// last, under its own heading.
export function helpGroups(everywhere: Scope, map?: Record<string, string[]>): HelpGroup[] {
  const bound = boundNow().filter((b) => !b.modal);
  // A page binds after the parts in it: the page's own come first.
  const scopes: Scope[] = [];
  for (const b of [...bound].reverse()) if (b.scope !== everywhere && !scopes.includes(b.scope)) scopes.push(b.scope);
  const on = (s: Scope) => s.commands.filter((c) => c.fixed || bound.some((b) => b.scope === s && !!b.handlers()[c.name]));
  const groups = scopes.map((s) => ({ title: s.title, keys: rowsOf(on(s), map) })).filter((g) => g.keys.length > 0);
  // Everywhere's are said whether bound or not: some are bound only where
  // they do something (the page's conversation), and they're everywhere's.
  return [...groups, { title: everywhere.title, keys: rowsOf(everywhere.commands, map) }];
}

// run is digits one after another as the run they are ("1–7"), where
// there are three or more of them.
function run(keys: string[]): string | undefined {
  if (keys.length < 3 || !keys.every((k, i) => /^[0-9]$/.test(k) && (i === 0 || Number(k) === Number(keys[i - 1]) + 1))) return undefined;
  return `${keys[0]}–${keys[keys.length - 1]}`;
}
