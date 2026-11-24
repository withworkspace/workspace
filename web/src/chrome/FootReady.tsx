import { Icon } from "../ui/Icon";
import { FootItem } from "./FootItem";
import css from "./FootReady.module.css";

// FootReady is how many of your pull requests are ready to merge
// (approved, or needing no review), as GitHub last said: a press goes to
// Reviews, where they are. Nothing's said while there are none.
export function FootReady({ count }: { count: number }) {
  if (count <= 0) return null;
  const words = `${count} ready to merge`;
  return (
    <FootItem to="/reviews" className={css.ready} title={`${words}: go to Reviews`}>
      <Icon name="check-circle" size={13} />
      <span>{words}</span>
    </FootItem>
  );
}
