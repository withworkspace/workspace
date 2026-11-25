import css from "./Keys.module.css";
import { cx } from "./cx";

// keysSaid is a key or a chord as it's read, a key at a time: "g m" is g
// then m, "mod+⏎" is ⌘⏎ on a Mac and Ctrl+⏎ elsewhere.
export function keysSaid(k: string): string[] {
  const mac = /Mac|iPhone|iPad/.test(navigator.platform);
  return k.split(" ").map((p) => p.replace(/^mod\+/, mac ? "⌘" : "Ctrl+").replace(/(^|\+|⌘)shift\+/, (_, before: string) => before + (mac ? "⇧" : "Shift+")));
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
