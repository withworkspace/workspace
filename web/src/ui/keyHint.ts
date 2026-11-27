import { usePrefs } from "../prefs";
import { keysSaid } from "./Keys";
import { saying, toast } from "./toast";

// A key hint is ws saying, in passing, that what you just pressed with
// the pointer has a key: the next time, it's quicker. It's said only
// where you've asked for it (Settings › Keyboard: off, unless turned
// on), by the library's parts that know their key: a Button, a
// ToggleButton, a Segment. They know it from keys= (a command, or a key
// written out), or from being
// given what useCommand made (keymap/useCommands.ts), which says its own.
//
// A key never comes through a part: it calls what it's bound to. So a
// part pressed is a part pressed, and the only thing to leave out is one
// pressed by the keys' focus (⏎ or space on it), which the part does:
// it calls this for the pointer alone.

// keyHint says keys is the key for what was pressed, if hints are on.
// A key that's still being said isn't said again over itself.
export function keyHint(keys: string) {
  if (!usePrefs.getState().keyHints) return;
  if (saying().some((t) => t.keys === keys)) return;
  toast("Keyboard shortcut:", { keys });
}

// titleWithKeys is what a part's tooltip says: what it does, then its
// key, "Review its changes (v)". With no title there's none: a key by
// itself isn't a tooltip (what has no title draws its key itself, or is
// plain enough without).
export function titleWithKeys(title: string | undefined, keys: string | undefined): string | undefined {
  if (!keys || !title) return title;
  return `${title} (${keysSaid(keys).join(" ")})`;
}

// pointed is whether a press was the pointer's: a mouse or a pen, which
// are used beside a keyboard. Not the keys' own (⏎ on what has the
// focus), a screen reader's, or a finger's.
export const pointed = (e: { pointerType: string }) => e.pointerType === "mouse" || e.pointerType === "pen";

// What a click says of how it was made: how many presses of the pointer
// (none: ⏎ or space on what has the focus, or a screen reader), and, in a
// browser that says, which kind of pointer.
type Click = { detail: number; nativeEvent?: unknown };

// clicked is whether a click was the pointer's, as pointed is of a press:
// for what isn't react-aria's, and has only the browser's click to go by.
export function clicked(e: Click): boolean {
  if (e.detail === 0) return false;
  const kind = (e.nativeEvent as { pointerType?: string } | undefined)?.pointerType;
  return !kind || pointed({ pointerType: kind });
}

// hintOnClick is a click handler that hints at keys, then does what
// onClick does: for what shows its key itself (Find's button, with ⌘K in
// it) and so has no title to say it in.
export function hintOnClick<E extends Click>(keys: string | undefined, onClick?: (e: E) => void): ((e: E) => void) | undefined {
  if (!keys) return onClick;
  return (e) => {
    if (clicked(e)) keyHint(keys);
    onClick?.(e);
  };
}

// keyProps is a key said and hinted by what isn't one of the library's
// parts: a link drawn as a button, a part's own button. It's the title
// and the click handler to give it:
//
//   <Link to={files} className={buttonClass()} {...keyProps("o", "Open it in Files")}>
//
// The title is the browser's, with the key after it; onClick is what the
// thing does when it's clicked besides, if anything. With no key, they're
// the title and the handler as given.
export function keyProps<E extends Click>(keys: string | undefined, title?: string, onClick?: (e: E) => void): { title: string | undefined; onClick: ((e: E) => void) | undefined } {
  return { title: titleWithKeys(title, keys), onClick: hintOnClick(keys, onClick) };
}
