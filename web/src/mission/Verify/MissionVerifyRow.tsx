import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import { Icon, type IconName } from "../../ui/Icon";
import { type Tone, toneClass } from "../../ui/tone";
import css from "./MissionVerifyRow.module.css";

const kinds = { wanted: css.wanted, approved: css.approved, settled: css.settled };

type MissionVerifyRowProps = {
  id?: string;
  // What it came to: an icon, in that verdict's tone.
  icon: IconName;
  tone: Tone;
  title: ReactNode;
  // What's said of it under its name: who says so, what shows it.
  line?: ReactNode;
  // It's the check that's open.
  selected?: boolean;
  // wanted: it's left for you, in a bundle that's yours to approve, and
  // stands out. approved (by you) and settled (without you) sit back: a
  // line each, its mark and its name.
  kind?: "wanted" | "approved" | "settled";
  onChoose: () => void;
};

// MissionVerifyRow is one check of the open bundle, in its list: what it
// came to, its name, and a line of what's said of it. Pressed, it opens
// beside the list.
export function MissionVerifyRow({ id, icon, tone, title, line, selected, kind, onChoose }: MissionVerifyRowProps) {
  return (
    <button id={id} type="button" className={cx(css.row, selected && css.selected, kind && kinds[kind])} aria-pressed={selected} onClick={onChoose}>
      <span className={cx(css.mark, toneClass(tone))}>
        <Icon name={icon} size={16} />
      </span>
      <span className={css.text}>
        <span className={css.title}>{title}</span>
        <span className={css.line}>{line}</span>
      </span>
    </button>
  );
}
