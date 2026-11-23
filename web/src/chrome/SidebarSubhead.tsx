import { cx } from "../ui/cx";
import css from "./SidebarSubhead.module.css";

type SidebarSubheadProps = Omit<React.ComponentProps<"h3">, "children"> & {
  // What the rows under it are: "To review".
  label: string;
};

// SidebarSubhead is a small heading within a list in the sidebar: the
// rows under it are of the list, and a kind of their own (in Needs you,
// the pull requests that ask for your review). In the rail it's a short
// rule, lighter than the list's own.
export function SidebarSubhead({ label, className, ...rest }: SidebarSubheadProps) {
  return (
    <h3 className={cx(css.subhead, className)} {...rest}>
      <span className={css.words}>{label}</span>
    </h3>
  );
}
