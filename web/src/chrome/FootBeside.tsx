import { useContextMenu } from "../ui/ContextMenu";
import { Icon } from "../ui/Icon";
import { Mark } from "../ui/Mark";
import { MenuItem, MenuText } from "../ui/Menu";
import css from "./FootBeside.module.css";
import { FootItem } from "./FootItem";

type FootBesideProps = {
  // Beside is open.
  open: boolean;
  // How many things are there to peek at while it's closed: its toggle is
  // marked then, since Beside never opens by itself.
  waiting?: number;
  onPress?: () => void;
  // Beside floats over the page, instead of the page making room; and
  // changing which, where it can be changed (not on a narrow window,
  // where it's a sheet either way). It's in the toggle's menu.
  over?: boolean;
  onOver?: (over: boolean) => void;
};

// FootBeside is Beside's toggle, at the foot's end, on the side Beside
// is: the sidebar's (FootSidebar) is at the foot's start, on its own.
// It's how Beside is closed, and its menu (the mouse's other button) has
// what else Beside does: float over the page, or dock next to it.
export function FootBeside({ open, waiting = 0, onPress, over = false, onOver }: FootBesideProps) {
  const marked = !open && waiting > 0;
  const menu = useContextMenu();
  const says = open ? "Close Beside (\\)" : marked ? `Beside: ${waiting === 1 ? "something" : `${waiting} things`} to peek at` : "Beside: a peek at a second thing, next to the page";
  const float = over ? "Dock it: the page makes room" : "Float it over the page";
  return (
    <FootItem
      on={open}
      label="Beside"
      title={onOver ? `${says}. Right-click for more` : says}
      onPress={onPress}
      onContextMenu={menu(
        "Beside",
        onOver &&
          (() => (
            <>
              <MenuItem id="float" textValue={float} onAction={() => onOver(!over)}>
                <Icon name="float" />
                <MenuText>{float}</MenuText>
              </MenuItem>
              <MenuItem id="toggle" textValue={open ? "Close Beside" : "Open Beside"} onAction={onPress}>
                <Icon name="panel" />
                <MenuText>{open ? "Close Beside" : "Open Beside"}</MenuText>
              </MenuItem>
            </>
          )),
      )}
    >
      <span className={css.icon}>
        <Icon name="panel" size={13} />
        {marked && <Mark ringed className={css.mark} />}
      </span>
    </FootItem>
  );
}
