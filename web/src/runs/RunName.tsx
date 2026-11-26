import { Link, type To } from "react-router";
import { cx } from "../ui/cx";
import css from "./RunName.module.css";
import { textClass } from "../ui/Text";

// RunName is what a run is called, in its line: its recipe's name, as
// it's typed. Given where to go (to), it's a link there.
export function RunName({ children, to, title, className }: { children: React.ReactNode; to?: To; title?: string; className?: string }) {
  const cls = cx(textClass("mono"), css.name, className);
  return to ? (
    <Link to={to} className={cls} title={title}>
      {children}
    </Link>
  ) : (
    <span className={cls} title={title}>
      {children}
    </span>
  );
}
