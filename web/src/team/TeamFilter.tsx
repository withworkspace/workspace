import type { Ref } from "react";
import { Input } from "../ui/Input";
import css from "./TeamFilter.module.css";

// TeamFilter is the Team page's filter, in its head: text, a repo or a
// person. ⏎ leaves it as it is, esc empties it; either gives the keys back
// to the page.
export function TeamFilter({ ref, value, onChange }: { ref?: Ref<HTMLInputElement>; value: string; onChange: (q: string) => void }) {
  return (
    <Input
      ref={ref}
      className={css.filter}
      value={value}
      placeholder="Filter: text, repo, person"
      aria-label="Filter what happened"
      onChange={(e) => onChange(e.target.value)}
      onKeyDown={(e) => {
        if (e.key === "Escape" || e.key === "Enter") {
          e.preventDefault();
          if (e.key === "Escape") onChange("");
          e.currentTarget.blur();
        }
      }}
    />
  );
}
