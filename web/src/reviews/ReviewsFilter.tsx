import type { Ref } from "react";
import { Input } from "../ui/Input";
import css from "./ReviewsFilter.module.css";

// ReviewsFilter filters the Reviews page's pull requests by text, repo or
// author. Esc clears it, and esc or ⏎ leave it for the list.
export function ReviewsFilter({ ref, value, onChange }: { ref?: Ref<HTMLInputElement>; value: string; onChange: (q: string) => void }) {
  return (
    <Input
      ref={ref}
      className={css.filter}
      value={value}
      placeholder="Filter: text, repo, author"
      aria-label="Filter pull requests"
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
