import css from "./Keys.module.css";
import { cx } from "./cx";

// What a key press is called (keyName, in keys.ts), as it's read.
const named: Record<string, string> = { Enter: "⏎", Escape: "esc", ArrowUp: "↑", ArrowDown: "↓", ArrowLeft: "←", ArrowRight: "→", Backspace: "⌫" };

// keysSaid is a key or a chord as it's read, a key at a time: "g m" is g
// then m, "mod+⏎" is ⌘⏎ on a Mac and Ctrl+⏎ elsewhere, "mod+alt+1" ⌘⌥1. A key written as
// a key map names it is read the same: "mod+Enter" is ⌘⏎, "Escape" esc.
export function keysSaid(k: string): string[] {
  const mac = /Mac|iPhone|iPad/.test(navigator.platform);
  return k.split(" ").map((p) =>
    p
      .replace(/^mod\+/, mac ? "⌘" : "Ctrl+")
      .replace(/(^|\+|⌘)alt\+/, (_, before: string) => before + (mac ? "⌥" : "Alt+"))
      .replace(/(^|\+|⌘|⌥)shift\+/, (_, before: string) => before + (mac ? "⇧" : "Shift+"))
      .replace(/[A-Z][A-Za-z]+$/, (name) => named[name] ?? name),
  );
}

// Keys shows a key or a chord (keysSaid). Each key is a kbd, whose look
// is the element's (styles.css), as it is in a document that writes one.
export function Keys({ k, className }: { k: string; className?: string }) {
  return (
    <span className={cx(css.keys, className)}>
      {keysSaid(k).map((p, i) => (
        <kbd key={i}>{p}</kbd>
      ))}
    </span>
  );
}
