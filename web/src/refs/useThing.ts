import { ApiError, getPluginThing, type PluginThing } from "../api";
import { useLoaded } from "../load";
import type { PageRef } from "./refs";

// What ws has of plugins' things, or was told when it asked, kept while
// the page is open: null is one its plugin has none of. That, and a plugin
// that couldn't be asked, are asked again the next time.
const things = new Map<string, Promise<PluginThing | null>>();
export const thingOf = (plugin: string, what: string, id: string) => {
  const key = `${plugin}\n${what}\n${id}`;
  let got = things.get(key);
  if (!got) {
    got = getPluginThing(plugin, what, id).catch((e: unknown) => {
      things.delete(key);
      if (e instanceof ApiError && e.status === 404) return null;
      throw e;
    });
    things.set(key, got);
  }
  return got;
};

// forgetThing drops what's kept of one thing, so it's asked for again the
// next time it's drawn: after something's been done to it.
export const forgetThing = (plugin: string, what: string, id: string) => things.delete(`${plugin}\n${what}\n${id}`);

// useThing is a plugin's thing as ws has it, or as its plugin says when
// asked: undefined while it's asked, null for one the plugin has none of,
// and error for a plugin that couldn't be asked.
export function useThing(r: Extract<PageRef, { kind: "thing" }> | undefined) {
  const { value: thing, error } = useLoaded(r ? () => thingOf(r.plugin, r.what, r.id) : null, [r?.plugin, r?.what, r?.id]);
  return { thing, error };
}
