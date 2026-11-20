import { Count } from "./Count";
import css from "./GroupHead.module.css";
import { cx } from "./cx";

type GroupHeadProps = {
  // count is how many the group has, in a Count after its label: a
  // number, or words ("3 of 12"). Left out when undefined.
  count?: React.ReactNode;
} & React.ComponentProps<"h2">;

// GroupHead is the heading of a group of rows on a page: a quiet label,
// how many there are beside it, sticky as the page scrolls.
export function GroupHead({ count, className, children, ...rest }: GroupHeadProps) {
  return (
    <h2 className={cx(css.head, className)} {...rest}>
      {children}
      {count !== undefined && (
        <>
          {" "}
          <Count>{count}</Count>
        </>
      )}
    </h2>
  );
}
