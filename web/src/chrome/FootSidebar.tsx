import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { FootItem } from "./FootItem";
import css from "./FootSidebar.module.css";

type FootSidebarProps = {
  // The sidebar is there (open or a rail), not hidden.
  shown: boolean;
  // How many things need you. It's said here only while the sidebar is
  // hidden: with its list out of sight, what can't move without you still
  // has to be somewhere in the window.
  count: number;
  // One has just arrived: the count lights up once, as its row would.
  fresh?: boolean;
  // Hiding the sidebar, or bringing it back.
  onPress?: () => void;
};

// FootSidebar is the sidebar's toggle, at the foot's start, on the side
// the sidebar is. While the sidebar is hidden and something needs you it
// says how many, in what needs you's colour, as part of the same thing to
// press: the way back to the list is the count of what's in it.
export function FootSidebar({
  shown,
  count,
  fresh,
  onPress,
}: FootSidebarProps) {
  const says = !shown && count > 0;
  const title = shown
    ? "Hide sidebar"
    : says
      ? `${count} ${count === 1 ? "needs" : "need"} you: show sidebar`
      : "Show sidebar";
  return (
    <FootItem
      on={shown}
      label={
        says
          ? `Sidebar: ${count} ${count === 1 ? "needs" : "need"} you`
          : "Sidebar"
      }
      title={title}
      onPress={onPress}
      className={cx(says && css.needs, says && fresh && css.fresh)}
    >
      <Icon name="panel" size={13} className={css.flipped} />
      {says && (
        <span>
          {count} {count === 1 ? "needs you" : "need you"}
        </span>
      )}
    </FootItem>
  );
}
