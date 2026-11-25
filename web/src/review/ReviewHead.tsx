import type { ReactNode } from "react";
import { Crumbs } from "../ui/Crumbs";
import css from "./ReviewHead.module.css";

// ReviewHead is a review's head: back to where it's from (esc), its name,
// what the diff is of (scope) and the review's controls, which keep
// together.
export function ReviewHead({ back, name, scope, actions }: { back: { to: string; label: string }; name: ReactNode; scope?: ReactNode; actions: ReactNode }) {
  return (
    <>
      <Crumbs className={css.crumbs} steps={[{ ...back, keys: "esc" }]} />
      <header className={css.head}>
        <h1>{name}</h1>
        {scope}
        <span className={css.actions}>{actions}</span>
      </header>
    </>
  );
}
