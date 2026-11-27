import { leaderWithin } from "../keys";
import { everywhereKeys } from "../shell/keys";
import { scopeOf } from "./commands";
import { keysOf } from "./keymap";

// frameKeys are the keys of ws's own that work in a plugin's frame as
// they do outside it. The frame is another document, so a key pressed
// there never reaches this one: the plugin's page is told these (the
// frame protocol's welcome), and passes each on as it's pressed. They're
// everywhere's commands that say frame (shell/keys.ts), with the keys
// they have now: yours, where you've set them.
//
// The protocol says three kinds: a letter with ⌘ or Ctrl held (k: Find),
// always; a key by itself (? and C), when nothing's being typed in; and a
// key that the next letter, a to z, comes after (g). A key that's none of
// those (⌘⇧ and a letter, ⌥ and one) isn't passed on. z (a peek at what
// the pointer's on) isn't either: what the pointer's on in a plugin's
// page is the plugin's to know.
//
// What a page's told is what the keys were when it opened: one set
// since is passed on once it's opened again.
export function frameKeys(map?: Record<string, string[]>) {
  const chords = new Set<string>();
  const bare = new Set<string>();
  const leaders = new Set<string>();
  for (const c of scopeOf(everywhereKeys).commands) {
    if (!c.frame) continue;
    for (const k of keysOf(c, map)) {
      const chord = /^mod\+(.)$/.exec(k);
      const seq = /^(\S) [a-z]$/.exec(k);
      if (chord) chords.add(chord[1]);
      else if (seq) {
        bare.add(seq[1]);
        leaders.add(seq[1]);
      } else if (k.length === 1) bare.add(k);
    }
  }
  return { chords: [...chords], bare: [...bare], leaders: [...leaders], within: leaderWithin };
}
