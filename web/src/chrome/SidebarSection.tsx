import type { ReactNode } from "react";
import { Count } from "../ui/Count";
import { cx } from "../ui/cx";
import css from "./SidebarSection.module.css";

type SidebarSectionProps = Omit<React.ComponentProps<"div">, "title"> & {
  // What it's a list of: "Needs you", "Focus".
  label: string;
  // How many, where that's worth saying beside its name.
  count?: number;
  // Beside its heading: a setting of the list's own.
  tool?: ReactNode;
};

// SidebarSection is a list in the sidebar under a small heading: Needs
// you, Focus. Its rows are SidebarRows. In the rail its heading is a
// rule, so the two lists fold the same way.
//
// It's a region by its role, not a <section>: styles.css spaces every
// <section> for the pages, unlayered, which a part's own styles can't
// undo.
export function SidebarSection({
  label,
  count,
  tool,
  className,
  children,
  ...rest
}: SidebarSectionProps) {
  return (
    <div
      role="region"
      aria-label={label}
      className={cx(css.section, className)}
      {...rest}
    >
      <h2 className={css.head}>
        <span className={css.words}>
          {label}
          {count !== undefined && <Count>{count}</Count>}
        </span>
        {tool && <span className={css.tool}>{tool}</span>}
      </h2>
      {children}
    </div>
  );
}
