import { useEffect, useMemo, useRef } from "react";
import { type Command, type CommandDef, type Commands, scopeOf } from "./commands";
import { bind, type Handler } from "./dispatch";
import { keyOf, useKeymap } from "./keymap";

export interface BindOpts {
  // While a modal's bound, only a modal's commands are done.
  modal?: boolean;
  // Read before what has the focus sees the key (everywhere's: ⌘K).
  first?: boolean;
  // Read after every other's (a plugin's actions, which are bound
  // everywhere they're for): the page's own keys are the page's.
  late?: boolean;
}

// What a scope's commands are bound to: a handler for each that's bound
// now. One that's undefined or false isn't, now; one that gives back
// pass (keymap/dispatch.ts) leaves the key to the next.
export type Handlers<D extends Record<string, CommandDef>> = { [K in keyof D]?: Handler | false | undefined };

// useCommands binds a scope's commands to what does them, for as long as
// the component is mounted: what's done is what was given last.
//
//   useCommands(teamKeys, { next: () => select(index + 1), summarise: searching ? undefined : summarise });
export function useCommands<D extends Record<string, CommandDef>>(commands: Commands<D>, handlers: Handlers<D>, opts: BindOpts = {}) {
  const current = useRef(handlers);
  current.current = handlers;
  const s = scopeOf(commands);
  const modal = !!opts.modal;
  const first = !!opts.first;
  const late = !!opts.late;
  useEffect(() => bind({ scope: s, handlers: () => current.current as Record<string, Handler | false | undefined>, modal, first, late }), [s, modal, first, late]);
}

// A Keyed function is one a key does too, and says which: what
// useCommand gives back. A button given it to press (ui/Button.tsx)
// reads the key off it, for its tooltip, and for the hint that there was
// a key when it's pressed with the pointer (ui/keyHint.ts).
export type Keyed<F> = F & { readonly keys?: string; readonly command?: Command };

// useCommand binds one command to fn, as useCommands does, and gives fn
// back saying the command and its key, to hand to the button that does
// the same: the key's said wherever the button is, and it's whichever
// it is now.
//
//   const close = useCommand(viewerKeys.close, () => setOpen(false));
//   <Button onClick={close} title="Close">
//
// While enabled is false the command isn't bound and the function says
// no key, though it still does what it does when called. It's the same
// function for as long as its key is, so it can be a prop or a
// dependency.
//
// It has to reach the button as it is: () => close() around it has no
// key to read. A button whose press isn't what the key does (the key
// acts on the row the cursor's on, the button on its own) is told the
// command itself: <Button keys={filesKeys.view}>.
export function useCommand(c: Command, fn: () => void, opts: BindOpts & { enabled?: boolean } = {}): Keyed<() => void> {
  const current = useRef(fn);
  current.current = fn;
  const on = opts.enabled !== false;
  const handlers = useRef<Record<string, Handler | undefined>>({});
  handlers.current = { [c.name]: on ? () => current.current() : undefined };
  const modal = !!opts.modal;
  const first = !!opts.first;
  useEffect(() => bind({ scope: c.scope, handlers: () => handlers.current, modal, first }), [c.scope, modal, first]);
  const map = useKeymap();
  const said = on ? keyOf(c, map) : undefined;
  return useMemo(() => Object.assign(() => current.current(), { keys: said, command: c }), [said, c]);
}
