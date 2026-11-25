import { type Prefs, usePref } from "../prefs";
import { keysSaid } from "./Keys";

// The key that sends what you've written in a box: a reply, a message to
// crew, a comment. It's yours to choose (Settings › Keyboard), and the
// same in every such box:
//
//   "mod+enter"  ⌘⏎ sends, and ⏎ is a new line
//   "enter"      ⏎ sends, and ⇧⏎ is a new line
//
// Some boxes send two ways (a cell's: change it, or only ask). The other
// way is the same key with one more held: ⌘⇧⏎ where ⌘⏎ sends, ⌘⏎ where ⏎
// does.
//
// A form with more than one box (a decision's, a review's verdict and
// summary) isn't one of these: ⌘⏎ confirms it whatever's chosen here, as
// ⏎ in one of its boxes is always a new line.
export type SendKey = Prefs["sendKey"];

type Press = { key: string; shiftKey: boolean; metaKey: boolean; ctrlKey: boolean };

// sent is whether a key press sends, with how as the key that does: no,
// or yes and which of the two ways.
export function sent(e: Press, how: SendKey): false | { other: boolean } {
  if (e.key !== "Enter") return false;
  const mod = e.metaKey || e.ctrlKey;
  if (how === "enter") return e.shiftKey ? false : { other: mod };
  return mod ? { other: e.shiftKey } : false;
}

// sendKeys is the keys that send, as Keys reads them ("mod+⏎"): the
// other way's with other.
export function sendKeys(how: SendKey, other = false): string {
  if (how === "enter") return other ? "mod+⏎" : "⏎";
  return other ? "mod+shift+⏎" : "mod+⏎";
}

// sendKeysSaid is those keys in words, for a title: "⌘⏎".
export const sendKeysSaid = (how: SendKey, other = false) => keysSaid(sendKeys(how, other)).join(" ");

// useSendKey is the key you've chosen.
export function useSendKey(): SendKey {
  return usePref("sendKey")[0];
}
