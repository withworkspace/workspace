// A command is something ws does that a key can do: "Next", "Go to
// Missions", "Summarise, or retry". It's named apart from its key, so
// the key can be another one (yours: Settings › Keyboard), and what says
// the key (a tooltip, the Keys dialog, a hint) says whichever it is.
//
// Commands come in scopes: the keys of a page, of a part (the picture
// viewer), or of everywhere. Each scope's are written once, in its
// area's keys.ts, with the keys they have unless you set others:
//
//   export const teamKeys = scope("team", "Team", {
//     next: { title: "Next", keys: ["j", "ArrowDown"], row: "Next / previous", moves: true },
//     prev: { title: "Previous", keys: ["k", "ArrowUp"], row: "Next / previous", moves: true },
//     summarise: { title: "Summarise, or retry", keys: "w" },
//   });
//
// and bound where they're done, by name (keymap/useCommands.ts):
//
//   useCommands(teamKeys, { next: () => select(index + 1), ... });
//
// Keys are written as a key map names a press (keyName, keys.ts): "j",
// "J", "Enter", "Escape", "mod+k" (⌘ on macOS, Ctrl elsewhere),
// "shift+ArrowDown", "alt+1"; one then another with a space: "g m".

export interface CommandDef {
  // What it does, as Settings and the Keys dialog say it.
  title: string;
  // Its keys, unless you've set others: the first is the one that's said.
  // [] is none until you give it one.
  keys: string | string[];
  // The Keys dialog's line for it, where it shares one: the commands
  // of a scope with the same row are one line, their keys side by side
  // ("j / k   Next / previous").
  row?: string;
  // It's done while something's being typed in a field, too (⌘K, ⌘⏎).
  // Escape always is.
  fields?: boolean;
  // A plugin's page passes it on, as it would be pressed here (the frame
  // protocol's welcome: keys.ts, frameKeys).
  frame?: boolean;
  // It moves the keyboard's cursor (j, k): the page shows the cursor once
  // it's pressed (shell/useKeyboarding.ts).
  moves?: boolean;
  // It isn't ws's to bind: what a part does with its own keys (an
  // editor's ``` or a tree's arrows). The Keys dialog says it; it can't
  // be set to another key.
  fixed?: boolean;
  // Keys it could have, which it hasn't until you say so (a plugin's
  // suggestion, in Settings › Keyboard: Use): never bound by themselves,
  // as two plugins may suggest the same.
  suggested?: string[];
}

export interface Command {
  // scope.name: "team.next".
  readonly id: string;
  readonly name: string;
  readonly scope: Scope;
  readonly title: string;
  readonly defaults: readonly string[];
  readonly row?: string;
  readonly fields: boolean;
  readonly frame: boolean;
  readonly moves: boolean;
  readonly fixed: boolean;
  readonly suggested: readonly string[];
}

export interface Scope {
  readonly id: string;
  // What Settings and the Keys dialog head its commands with.
  readonly title: string;
  readonly commands: readonly Command[];
}

const scopeKey: unique symbol = Symbol("scope");

// Commands is what scope gives back: each command by its name, and the
// scope itself (scopeOf).
export type Commands<D extends Record<string, CommandDef> = Record<string, CommandDef>> = { readonly [K in keyof D]: Command } & { readonly [scopeKey]: Scope };

const scopes = new Map<string, Scope>();

// scope makes a scope's commands, from what each is and does by default.
// Its id is unique: one made again under the same id (a module read
// again, in development) takes the old one's place.
export function scope<D extends Record<string, CommandDef>>(id: string, title: string, defs: D): Commands<D> {
  const commands: Command[] = [];
  const s: Scope = { id, title, commands };
  const out: Record<string, Command> = {};
  for (const [name, d] of Object.entries(defs)) {
    const c: Command = {
      id: `${id}.${name}`,
      name,
      scope: s,
      title: d.title,
      defaults: typeof d.keys === "string" ? [d.keys] : [...d.keys],
      row: d.row,
      fields: !!d.fields,
      frame: !!d.frame,
      moves: !!d.moves,
      fixed: !!d.fixed,
      suggested: d.suggested ?? [],
    };
    commands.push(c);
    out[name] = c;
  }
  scopes.set(id, s);
  return Object.assign(out, { [scopeKey]: s }) as Commands<D>;
}

// forgetScope takes a scope away: one made for what's gone (a plugin
// that's been removed).
export function forgetScope(id: string) {
  scopes.delete(id);
}

// scopeOf is the scope commands are of.
export const scopeOf = (c: Commands): Scope => c[scopeKey];

// allScopes is every scope there is, once keymap/all.ts has read each
// area's keys, in the order they were made.
export const allScopes = (): Scope[] => [...scopes.values()];

// commandById is the command with an id, if there's one.
export function commandById(id: string): Command | undefined {
  const dot = id.indexOf(".");
  return scopes.get(id.slice(0, dot))?.commands.find((c) => c.name === id.slice(dot + 1));
}

// isCommand is whether what's given is a command (and not a key written
// out, as a part's keys= can be).
export const isCommand = (k: unknown): k is Command => typeof k === "object" && k !== null && "scope" in k && "defaults" in k;
