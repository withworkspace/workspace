import { useSyncExternalStore } from "react";

// Whether What's new has been asked for, from Settings' About: then it
// shows everything this ws says is new, until it's closed.

let asked = false;
const listeners = new Set<() => void>();

const told = () => listeners.forEach((l) => l());

// openWhatsNew shows everything this ws says is new.
export function openWhatsNew() {
  asked = true;
  told();
}

// closeAsked is it closed again.
export function closeAsked() {
  asked = false;
  told();
}

// useAsked is whether it's asked for now.
export function useAsked(): boolean {
  return useSyncExternalStore(
    (l) => {
      listeners.add(l);
      return () => listeners.delete(l);
    },
    () => asked,
  );
}
