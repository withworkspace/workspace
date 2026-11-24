import { Keys } from "../../ui/Keys";
import css from "./FindFootKey.module.css";

// FindFootKey is a key and what it does from here, in Find's foot.
export function FindFootKey({ k, children }: { k: string; children: string }) {
  return (
    <span>
      <Keys className={css.keys} k={k} />
      {` ${children}`}
    </span>
  );
}
