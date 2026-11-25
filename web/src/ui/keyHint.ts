import { usePrefs } from "../prefs";
import { keysSaid } from "./Keys";
import { getToasts, toast } from "./toast";

// A key hint is ws saying, in passing, that what you just pressed with
// the pointer has a key: the next time, it's quicker. It's said only
// where you've asked for it (Settings › Keyboard: off, unless turned
// on), by the library's parts that know their key: a Button, a
// ToggleButton, a Segment. They know it from keys="v", or from being
// given what useShortcut made (keys.ts), which says its own.
//
// A key never comes through a part: it calls what it's bound to. So a
// part pressed is a part pressed, and the only thing to leave out is one
// pressed by the keys' focus (⏎ or space on it), which the part does:
// it calls this for the pointer alone.

// keyHint says keys is the key for what was pressed, if hints are on.
// A key that's still being said isn't said again over itself.
export function keyHint(keys: string) {
  if (!usePrefs.getState().keyHints) return;
  if (getToasts().some((t) => t.keys === keys)) return;
  toast("There's a key for that:", { keys });
}

// titleWithKeys is what a part's tooltip says: what it does, then its
// key, "Review its changes (v)". Either alone, where that's all there is.
export function titleWithKeys(title: string | undefined, keys: string | undefined): string | undefined {
  if (!keys) return title;
  const said = keysSaid(keys).join(" ");
  return title ? `${title} (${said})` : said;
}

// pointed is whether a press was the pointer's: a mouse or a pen, which
// are used beside a keyboard. Not the keys' own (⏎ on what has the
// focus), a screen reader's, or a finger's.
export const pointed = (e: { pointerType: string }) => e.pointerType === "mouse" || e.pointerType === "pen";
