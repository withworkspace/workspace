import type { RefObject } from "react";
import { cx } from "../../ui/cx";
import { Icon } from "../../ui/Icon";
import type { Section } from "../reading";
import css from "./ReaderPill.module.css";
import type { Reading } from "./useReading";

// ReaderPill is the contents where the rail doesn't fit: a bar stuck to
// the top once you've scrolled past the title, saying which section you're
// in and how far through you are. It opens the contents (as t does).
export function ReaderPill({ title, sections, reading, open, toggle, pill }: { title: string; sections: Section[]; reading: Reading; open: boolean; toggle: () => void; pill?: RefObject<HTMLButtonElement | null> }) {
  const at = sections[reading.at];
  return (
    <div className={cx(css.wrap, reading.past && css.shown)}>
      <button type="button" ref={pill} className={css.pill} data-contents-toggle aria-expanded={open} aria-hidden={!reading.past || undefined} tabIndex={reading.past ? undefined : -1} title="Its contents (t)" onClick={toggle}>
        <span className={css.doc}>{title}</span>
        <Icon name="chevron-right" size={12} />
        <span className={css.at}>{at ? at.text : "Top"}</span>
        <Icon name="chevron-down" size={12} />
        <span className={css.spacer} />
        <span className={css.pct}>{reading.pct}%</span>
        <span className={css.line} style={{ width: `${reading.pct}%` }} />
      </button>
    </div>
  );
}
