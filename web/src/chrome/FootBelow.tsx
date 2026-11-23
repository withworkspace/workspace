import { Icon } from "../ui/Icon";
import { FootItem } from "./FootItem";

type FootBelowProps = {
  // Below is open.
  open: boolean;
  // What Below is for, said when its toggle is pointed at.
  title?: string;
  onPress?: () => void;
};

// FootBelow is Below's toggle, at the foot's end before Beside's: the
// two in the order their docks are in the window.
export function FootBelow({ open, title = "Below: what's going on, under the page", onPress }: FootBelowProps) {
  return (
    <FootItem on={open} label="Below" title={open ? "Close Below" : title} onPress={onPress}>
      <Icon name="fold" size={13} />
    </FootItem>
  );
}
