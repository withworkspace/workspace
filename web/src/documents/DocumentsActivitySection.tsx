import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import { GroupHead } from "../ui/GroupHead";
import css from "./DocumentsActivitySection.module.css";

type DocumentsActivitySectionProps = {
  label: string; // what it's called, to a screen reader
  heading: ReactNode;
  count: number;
  title?: string; // its heading's tooltip
  flat?: boolean; // in DocumentsActivityFold: its heading doesn't stick
  children: ReactNode;
};

// DocumentsActivitySection is what's happening in a space, or across
// them, of one sort: what's in flight, or what changed lately.
export function DocumentsActivitySection({ label, heading, count, title, flat, children }: DocumentsActivitySectionProps) {
  return (
    <section aria-label={label}>
      <GroupHead className={cx(css.head, flat && css.flat)} title={title} count={count}>
        {heading}
      </GroupHead>
      {children}
    </section>
  );
}
