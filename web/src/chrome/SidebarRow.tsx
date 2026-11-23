import type { ReactNode } from "react";
import { Link } from "react-router";
import { cx } from "../ui/cx";
import css from "./SidebarRow.module.css";

type SidebarRowProps = Omit<
  React.ComponentProps<typeof Link>,
  "to" | "title" | "children"
> & {
  to: string;
  // Before its name: how it stands (a Status), or its kind's icon.
  lead: ReactNode;
  // What it is, and what's said of it when it's pointed at.
  title: string;
  says?: string;
  // The page you're on is its own.
  on?: boolean;
  // It has just arrived: it lights up once, where it is.
  fresh?: boolean;
  // Over its end while it's pointed at: something to do to the row.
  end?: ReactNode;
};

// SidebarRow is a row of a list in the sidebar: something that needs
// you, something you pinned. Its icon and its name, a press from its
// page. In the rail it's its icon.
export function SidebarRow({
  to,
  lead,
  title,
  says,
  on,
  fresh,
  end,
  className,
  ...rest
}: SidebarRowProps) {
  return (
    <Link
      to={to}
      title={says ?? title}
      aria-current={on ? "page" : undefined}
      className={cx(css.row, on && css.on, fresh && css.fresh, className)}
      {...rest}
    >
      {lead}
      <span className={css.words}>{title}</span>
      {end}
    </Link>
  );
}
