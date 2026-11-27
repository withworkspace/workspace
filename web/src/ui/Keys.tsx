import { type KeyLike, useSaid } from "../keymap/keymap";
import css from "./Keys.module.css";
import { cx } from "./cx";

// What a key press is called (keyName, in keys.ts), as it's read.
const named: Record<string, string> = { Enter: "⏎", Escape: "esc", ArrowUp: "↑", ArrowDown: "↓", ArrowLeft: "←", ArrowRight: "→", Backspace: "⌫", Space: "␣" };

// keysSaid is a key or a chord as it's read, a key at a time: "g m" is g
// then m, "mod+⏎" is ⌘⏎ on a Mac and Ctrl+⏎ elsewhere, "mod+alt+1" ⌘⌥1,
// "mod+shift+f" ⌘⇧F, "W" ⇧W. A key written as
// a key map names it is read the same: "mod+Enter" is ⌘⏎, "Escape" esc.
export function keysSaid(k: string): string[] {
  const mac = /Mac|iPhone|iPad/.test(navigator.platform);
  return k.split(" ").map((p) =>
    p
      .replace(/^mod\+/, mac ? "⌘" : "Ctrl+")
      .replace(/(^|\+|⌘)alt\+/, (_, before: string) => before + (mac ? "⌥" : "Alt+"))
      .replace(/(^|\+|⌘|⌥)shift\+/, (_, before: string) => before + (mac ? "⇧" : "Shift+"))
      .replace(/[A-Z][A-Za-z]+$/, (name) => named[name] ?? name)
      // A letter held with a modifier is written big, as on the key (⌘K),
      // and a big one by itself says it's shift and the letter (⇧W).
      .replace(/([+⌘⌥⇧])([a-z])$/, (_, before: string, letter: string) => before + letter.toUpperCase())
      .replace(/^([A-Z])$/, "⇧$1"),
  );
}

// Keys shows a key or a chord (keysSaid). Each key is a kbd, whose look
// is the element's (styles.css), as it is in a document that writes one.
// Given a command, it's the command's key now (yours, where you've set
// one), and nothing where it has none.
export function Keys({ k: given, className }: { k: KeyLike; className?: string }) {
  const k = useSaid(given);
  if (!k) return null;
  return (
    <span className={cx(css.keys, className)}>
      {keysSaid(k).map((p, i) => (
        <kbd key={i}>{p}</kbd>
      ))}
    </span>
  );
}
