import { Button } from "../ui/Button";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import css from "./WorkspaceName.module.css";
import { WorkspaceMark } from "./WorkspaceMark";

type WorkspaceNameProps = {
  name: string;
  // Where the workspace is, said when its name is pointed at.
  root?: string;
  // What in its menu wants you, if anything: a place moved there that has
  // a count. It's said by a dot on the name.
  wants?: string;
  className?: string;
};

// WorkspaceName is the workspace's name as a button, which opens its
// menu: a button that looks like the name. It's a react-aria button, so a
// MenuTrigger round it gives it the menu. In the rail its name is cut
// off, leaving its mark.
export function WorkspaceName({
  name,
  root,
  wants,
  className,
}: WorkspaceNameProps) {
  // data-field: keys typed in the menu are its own, not the page's (keys.ts).
  return (
    <Button
      variant="bare"
      data-field
      aria-label={`${name}: workspace menu`}
      className={cx(css.name, className)}
    >
      <WorkspaceMark name={name} />
      <span className={css.words} title={root}>
        {name}
      </span>
      <Icon name="chevron-down" size={12} className={css.more} />
      {wants && (
        <span
          role="img"
          className={css.dot}
          title={wants}
          aria-label="Something in the menu needs you"
        />
      )}
    </Button>
  );
}
