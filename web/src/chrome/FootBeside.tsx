import { Icon } from "../ui/Icon";
import { Mark } from "../ui/Mark";
import css from "./FootBeside.module.css";
import { FootItem } from "./FootItem";

type FootBesideProps = {
  // Beside is open.
  open: boolean;
  // How many things are there to peek at while it's closed: its toggle is
  // marked then, since Beside never opens by itself.
  waiting?: number;
  onPress?: () => void;
};

// FootBeside is Beside's toggle, at the foot's end, on the side Beside
// is: the sidebar's (FootSidebar) is at the foot's start, on its own.
export function FootBeside({ open, waiting = 0, onPress }: FootBesideProps) {
  const marked = !open && waiting > 0;
  return (
    <FootItem on={open} label="Beside" title={open ? "Close Beside" : marked ? `Beside: ${waiting === 1 ? "something" : `${waiting} things`} to peek at` : "Beside: a peek at a second thing, next to the page"} onPress={onPress}>
      <span className={css.icon}>
        <Icon name="panel" size={13} />
        {marked && <Mark ringed className={css.mark} />}
      </span>
    </FootItem>
  );
}
