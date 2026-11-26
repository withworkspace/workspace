import { Icon } from "../ui/Icon";
import { Mark } from "../ui/Mark";
import css from "./FootBelow.module.css";
import { FootItem } from "./FootItem";

type FootBelowProps = {
  // Below is open.
  open: boolean;
  // How many services are running: said beside its icon, where any are.
  count?: number;
  // One has started since Below was last closed: marked, while it's
  // closed, since nothing opens it by itself.
  fresh?: boolean;
  onPress?: () => void;
};

// FootBelow is Below's toggle, at the foot's end before Beside's: the
// two in the order their docks are in the window. Below holds runs, so
// it's Runs: its icon, how many services are running, and a mark when
// one has started since you last looked.
export function FootBelow({ open, count = 0, fresh, onPress }: FootBelowProps) {
  const marked = !open && !!fresh;
  const running = count > 0 ? `${count} running` : "nothing running";
  const title = open ? "Close Runs" : marked ? `Runs: ${running}, one just started` : `Runs: ${running}`;
  return (
    <FootItem on={open} label={count > 0 ? `Runs: ${count} running` : "Runs"} title={title} onPress={onPress}>
      <span className={css.icon}>
        <Icon name="play" size={13} />
        {marked && <Mark ringed className={css.mark} />}
      </span>
      {count > 0 && <span>{count}</span>}
    </FootItem>
  );
}
