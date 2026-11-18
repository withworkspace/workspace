import { useSyncExternalStore } from "react";

// Whether the plugin's app in the frame has a backdrop up (backdrop.ts).
// The app's backdrop can only dim the frame, so while it has one the shell
// dims what's beside the frame, in the same scrim, and the two read as
// one. The frame's page says it (Plugin.tsx) and the shell draws it
// (App.tsx): they're far apart in the tree, and the dim has to be drawn in
// the same turn the app's backdrop came or went, so it's kept here and
// read as it changes (useSyncExternalStore renders at once, not when
// React next gets to it).

let dim = false;
const heard = new Set<() => void>();

// setFrameDim is the frame's page saying whether the app has a backdrop up.
export function setFrameDim(on: boolean): void {
  if (on === dim) return;
  dim = on;
  for (const fn of [...heard]) fn();
}

const hear = (fn: () => void) => {
  heard.add(fn);
  return () => void heard.delete(fn);
};

// useFrameDim is whether to dim what's beside the frame.
export function useFrameDim(): boolean {
  return useSyncExternalStore(hear, () => dim);
}
