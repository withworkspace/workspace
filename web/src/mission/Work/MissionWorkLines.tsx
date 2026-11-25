import { cx } from "../../ui/cx";
import css from "./MissionWorkLines.module.css";
import { Text } from "../../ui/Text";

// MissionWorkLines is how many lines were added and taken away: +12 −3.
// tally is one at the end of a file's line, or a folder's, in the list of
// what's changed.
export function MissionWorkLines({ additions, deletions, tally }: { additions: number; deletions: number; tally?: boolean }) {
  return (
    <span className={cx(css.lines, tally && css.tally)}>
      {additions > 0 && <Text plus>+{additions.toLocaleString()}</Text>}
      {additions > 0 && deletions > 0 && " "}
      {deletions > 0 && <Text minus>−{deletions.toLocaleString()}</Text>}
    </span>
  );
}
