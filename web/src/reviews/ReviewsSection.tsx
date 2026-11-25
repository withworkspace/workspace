import type { ReactNode } from "react";
import { Button } from "../ui/Button";
import { Count } from "../ui/Count";
import { GroupHead } from "../ui/GroupHead";
import { Icon } from "../ui/Icon";
import { Rows } from "../ui/Rows";
import css from "./ReviewsSection.module.css";

// ReviewsSection is a section of the Reviews page: its title and how many
// are in it, then its rows (a listbox). One that folds (drafts, bots) has
// its title as the button that opens it. The first of yours says so
// above it (yoursFirst).
export function ReviewsSection({ title, count, folds, open, yoursFirst, onFold, children }: { title: string; count: number; folds?: boolean; open: boolean; yoursFirst?: boolean; onFold?: () => void; children: ReactNode }) {
  return (
    <section aria-label={title} className={css.section}>
      {yoursFirst && <div className={css.divide}>Yours</div>}
      <GroupHead>
        {folds ? (
          <Button variant="link" className={css.fold} aria-expanded={open} onClick={onFold}>
            <Icon name={open ? "chevron-down" : "chevron-right"} size={12} /> {title}
          </Button>
        ) : (
          title
        )}{" "}
        <Count>{count}</Count>
      </GroupHead>
      {open && (
        <Rows role="listbox" aria-label={title}>
          {children}
        </Rows>
      )}
    </section>
  );
}
