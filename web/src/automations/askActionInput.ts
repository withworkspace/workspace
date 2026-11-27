import type { AutomationAction } from "../api";

// askActionInput asks what an action asks before it runs, in a dialog of
// the page's own (ActionInputAsker draws it): what you gave, by field, or
// undefined if it was put away. on is what it's run on, for the dialog to
// say.

export interface InputAsk {
  action: AutomationAction;
  on?: string;
  resolve: (input: Record<string, unknown> | undefined) => void;
}

let asking: InputAsk | undefined;
const listeners = new Set<() => void>();

export function askActionInput(action: AutomationAction, on?: string): Promise<Record<string, unknown> | undefined> {
  return new Promise((resolve) => {
    asking?.resolve(undefined);
    asking = { action, on, resolve };
    listeners.forEach((l) => l());
  });
}

// inputAsked is what's being asked now, if anything; answered puts it
// away, with what was given.
export const inputAsked = () => asking;
export function answered(input: Record<string, unknown> | undefined) {
  const ask = asking;
  asking = undefined;
  listeners.forEach((l) => l());
  ask?.resolve(input);
}
export function onInputAsked(l: () => void) {
  listeners.add(l);
  return () => listeners.delete(l);
}
