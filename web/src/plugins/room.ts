import { useSyncExternalStore } from "react";

// Which plugin's page wants the room (ws:room): a canvas, say, that's
// better the wider it is. The shell folds its sidebar to a rail while it
// does (App.tsx), as on ws's own such pages (shell/room.ts), and only on
// that plugin's pages. The frame's page says it (Plugin.tsx) and the
// shell folds (App.tsx): far apart in the tree, so it's kept here. It's
// forgotten when the page goes.

let room: string | undefined;
const heard = new Set<() => void>();

// setFrameRoom is a plugin's page saying whether it wants the room. A
// page that doesn't, of a plugin whose page isn't the one that last said
// it does, leaves that one's: a page going as another comes doesn't undo
// the new one's.
export function setFrameRoom(plugin: string, wants: boolean): void {
  const next = wants ? plugin : room === plugin ? undefined : room;
  if (next === room) return;
  room = next;
  for (const fn of [...heard]) fn();
}

const hear = (fn: () => void) => {
  heard.add(fn);
  return () => void heard.delete(fn);
};

// frameRoom is the plugin whose page wants the room, if one does.
export const frameRoom = (): string | undefined => room;

// useFrameRoom is the plugin whose page wants the room, if one does.
export function useFrameRoom(): string | undefined {
  return useSyncExternalStore(hear, () => room);
}
