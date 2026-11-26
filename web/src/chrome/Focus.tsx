import { cx } from "../ui/cx";
import css from "./Focus.module.css";
import { SidebarSection } from "./SidebarSection";

type FocusProps = Omit<React.ComponentProps<"div">, "title"> & {
  // How many pins it has.
  count: number;
  // A link to something pinnable is being dragged from the page: Focus
  // is there to drop it on, even with nothing pinned.
  bringing?: boolean;
};

// Focus is your pins, in the sidebar: a FocusRow each, drawn as Needs you
// is, so the rail folds them the same way. With none it isn't there,
// unless something's being dragged to it. What's dragged and dropped is
// its place's to handle (the section's drag events); data-focus is how
// that finds a drag that started in here.
export function Focus({
  count,
  bringing,
  className,
  children,
  ...rest
}: FocusProps) {
  if (!count && !bringing) return null;
  return (
    <SidebarSection
      data-focus=""
      label="Focus"
      count={count || undefined}
      className={cx(bringing && css.bringing, className)}
      {...rest}
    >
      {children}
      {bringing && !count && (
        <div className={css.drop}>Drop here to pin</div>
      )}
    </SidebarSection>
  );
}
