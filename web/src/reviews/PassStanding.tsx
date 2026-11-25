import type { ReactNode } from "react";
import type { PassMark } from "../api";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import { standingIcon } from "./passWords";
import css from "./PassStanding.module.css";

// The colour a pass's standing is said in: go (with notes too) green, no
// go (or couldn't run) red, running in the accent, the rest quiet.
const tone: Partial<Record<PassMark["standing"]["status"], string>> = { go: css.go, notes: css.go, nogo: css.nogo, failed: css.nogo, running: css.running };

// PassStanding is how a pass stands, as a soft chip in its colour: its
// icon, a spinner while it runs (spin), and what's said (children). One
// of the commit before (moved) is dimmed.
export function PassStanding({ standing, spin, moved, size = 13, title, children }: { standing: PassMark["standing"]; spin?: boolean; moved?: boolean; size?: number; title?: string; children: ReactNode }) {
  return (
    <span className={cx(css.standing, tone[standing.status], moved && css.moved)} title={title}>
      {spin ? <Spinner size={size} /> : <Icon name={standingIcon(standing)} size={size} />} {children}
    </span>
  );
}
