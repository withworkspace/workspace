import type { ComponentProps, ReactNode } from "react";
import { Link } from "react-router";
import { Button } from "./Button";
import { cx } from "./cx";
import css from "./MoreLink.module.css";

type MoreLinkProps = {
  children: ReactNode;
  // The page's own class, to place it.
  className?: string;
} & (
  | // Somewhere to go.
  { to: string; onClick?: undefined }
  // Something done here: the rest shown, another tab.
  | ({ to?: undefined } & Pick<ComponentProps<typeof Button>, "onClick" | "title" | "aria-expanded">)
);

// MoreLink is words in the accent that lead to the rest of something:
// "3 more in Work", "The whole log", "Show all". It's a link where the
// rest is somewhere else (to), and a button where it's shown here.
export function MoreLink({ to, className, children, ...rest }: MoreLinkProps) {
  if (to != null)
    return (
      <Link to={to} className={cx(css.more, className)}>
        {children}
      </Link>
    );
  return (
    <Button variant="link" className={cx(css.more, className)} {...rest}>
      {children}
    </Button>
  );
}
