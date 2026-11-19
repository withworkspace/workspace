import { useSyncExternalStore } from "react";

// What a plugin's page says it shows beyond its address (ws:here): an
// issue open in a panel over the board, by one of ws's names. Find starts
// from it on that plugin's pages, over what the address says. The frame's
// page says it (Plugin.tsx) and Find reads it (App.tsx), far apart in the
// tree, so it's kept here. It's forgotten when the page goes: the frame
// loaded again, another page connected, or ws left the plugin's pages.

export interface FrameHere {
  plugin: string;
  name: string;
}

let here: FrameHere | undefined;
const heard = new Set<() => void>();

// setFrameHere is a plugin's page saying the name it shows now, or none.
// None from a plugin whose page isn't the one that said the last leaves
// that one's: a page going as another comes doesn't forget the new one's.
export function setFrameHere(plugin: string, name: string | undefined): void {
  const next = name ? { plugin, name } : here?.plugin === plugin ? undefined : here;
  if (next === here || (next && here && next.plugin === here.plugin && next.name === here.name)) return;
  here = next;
  for (const fn of [...heard]) fn();
}

const hear = (fn: () => void) => {
  heard.add(fn);
  return () => void heard.delete(fn);
};

// frameHere is what the plugin's page said it shows now, if it said.
export const frameHere = (): FrameHere | undefined => here;

// useFrameHere is what the plugin's page said it shows, if it said.
export function useFrameHere(): FrameHere | undefined {
  return useSyncExternalStore(hear, () => here);
}
