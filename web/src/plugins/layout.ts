import type { V1 } from "@withworkspace/ui-lib/protocol";
import { useSyncExternalStore } from "react";

// Which plugin's page asked for ws's chrome expanded (ws:chrome-layout):
// a canvas, say, that's better the wider it is. The shell folds its
// sidebar to a rail while it is (App.tsx), as on ws's own such pages
// (shell/room.ts), and only on that plugin's pages. The frame's page says
// it (Plugin.tsx) and the shell folds (App.tsx): far apart in the tree,
// so it's kept here. It's standard again when the page goes.

let expanded: string | undefined;
const heard = new Set<() => void>();

// setFrameLayout is a plugin's page saying how it wants the chrome laid
// out. Standard, from a plugin whose page isn't the one that last asked
// for expanded, leaves that one's: a page going as another comes doesn't
// undo the new one's.
export function setFrameLayout(plugin: string, mode: V1.ChromeMode): void {
  const next = mode === "expanded" ? plugin : expanded === plugin ? undefined : expanded;
  if (next === expanded) return;
  expanded = next;
  for (const fn of [...heard]) fn();
}

const hear = (fn: () => void) => {
  heard.add(fn);
  return () => void heard.delete(fn);
};

// frameExpanded is the plugin whose page asked for the chrome expanded, if
// one did.
export const frameExpanded = (): string | undefined => expanded;

// useFrameExpanded is the plugin whose page asked for the chrome
// expanded, if one did.
export function useFrameExpanded(): string | undefined {
  return useSyncExternalStore(hear, () => expanded);
}
