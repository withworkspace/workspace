import { Keys } from "../ui/Keys";
import css from "./WindowKeyLabel.module.css";

// WindowKeyLabel draws a key as the key lists write it: "j / k" and "⏎ or
// o" are alternatives, "g m" a sequence, "mod+K" a chord.
export function WindowKeyLabel({ k }: { k: string }) {
  const alts = k.split(/\s+(?:\/|or)\s+/);
  return (
    <span className={css.label}>
      {alts.map((a, i) => (
        <span key={i}>
          {i > 0 && <span className="dim"> or </span>}
          <Keys k={a} />
        </span>
      ))}
    </span>
  );
}
