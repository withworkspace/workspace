import type { PassMark as Mark } from "../api";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import { standingIcon } from "./passWords";
import css from "./PassMark.module.css";

const tone: Partial<Record<Mark["standing"]["status"], string>> = { go: css.go, notes: css.go, nogo: css.nogo, failed: css.nogo };

// PassMark is how a pass stands, as a mark in its colour: a spinner while
// it runs.
export function PassMark({ standing, running, title }: { standing: Mark["standing"]; running: boolean; title: string }) {
  return (
    <span className={cx(css.mark, running ? css.running : tone[standing.status])} title={title}>
      {running ? <Spinner size={11} /> : <Icon name={standingIcon(standing)} size={11} />}
    </span>
  );
}
