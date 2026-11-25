import type { ThreadKind } from "../api";
import { Chip } from "../ui/Chip";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { kindIcon, kindWords } from "./words";
import css from "./AnnotateKindChip.module.css";

const kinds: Record<ThreadKind, string> = { ask: css.ask, change: css.change, note: css.note };

// AnnotateKindChip says what kind a thread is (ask, change, note), in its
// kind's colour.
export function AnnotateKindChip({ kind }: { kind: ThreadKind }) {
  return (
    <Chip className={cx(css.chip, kinds[kind])}>
      <Icon name={kindIcon[kind]} size={11} />
      {kindWords[kind]}
    </Chip>
  );
}
