import css from "./Keys.module.css";
import { cx } from "./cx";

// Keys shows a key or a chord: "g m" is g then m, "mod+⏎" is ⌘⏎ on a Mac
// and Ctrl+⏎ elsewhere. Each key is a kbd, whose look is the element's
// (styles.css), as it is in a document that writes one.
export function Keys({ k, className }: { k: string; className?: string }) {
  const mac = /Mac|iPhone|iPad/.test(navigator.platform);
  const parts = k.split(" ").map((p) => p.replace(/^mod\+/, mac ? "⌘" : "Ctrl+").replace(/(^|\+|⌘)shift\+/, (_, before: string) => before + (mac ? "⇧" : "Shift+")));
  return (
    <span className={cx(css.keys, className)}>
      {parts.map((p, i) => (
        <kbd key={i}>{p}</kbd>
      ))}
    </span>
  );
}
