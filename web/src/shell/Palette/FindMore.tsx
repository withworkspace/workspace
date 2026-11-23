import { Keys } from "../../ui/Keys";
import css from "./FindMore.module.css";
import { FindNote } from "./FindNote";

// FindMore says how many more of a group there are than its rows show,
// and, with narrows, the key that narrows Find to it.
export function FindMore({ count, narrows }: { count: number; narrows?: boolean }) {
  return (
    <FindNote>
      {count} more
      {narrows && <Keys k="⇥" className={css.keys} />}
    </FindNote>
  );
}
