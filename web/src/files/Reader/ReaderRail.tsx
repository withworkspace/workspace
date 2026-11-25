import { useEffect, useRef } from "react";
import { cx } from "../../ui/cx";
import { Icon } from "../../ui/Icon";
import type { Section } from "../reading";
import css from "./ReaderRail.module.css";
import type { Reading } from "./useReading";

// ReaderRail is the contents beside the text: "On this page", how far
// through you are, and each section, the one you're in marked and those
// you've passed less faint. Hidden (t), it keeps its room, so the text
// doesn't move.
export function ReaderRail({ sections, reading, hidden, go }: { sections: Section[]; reading: Reading; hidden: boolean; go: (anchor: string) => void }) {
  const box = useRef<HTMLElement>(null);
  // In a long list, the section you're in stays in sight.
  useEffect(() => {
    const rail = box.current;
    const on = rail?.querySelector<HTMLElement>('[aria-current="location"]');
    if (!rail || !on) return;
    if (on.offsetTop < rail.scrollTop || on.offsetTop > rail.scrollTop + rail.clientHeight - 40) rail.scrollTop = on.offsetTop - rail.clientHeight / 2;
  }, [reading.at]);
  return (
    <aside className={cx(css.rail, hidden && css.hidden)} ref={box} aria-label="On this page" aria-hidden={hidden || undefined}>
      <div className={css.head}>
        <Icon name="list" size={12} /> On this page <span className={css.pct}>{reading.pct}%</span>
      </div>
      <div className={css.track} aria-hidden>
        <i style={{ width: `${reading.pct}%` }} />
      </div>
      {sections.length > 0 ? (
        <nav className={css.list}>
          {sections.map((s, i) => (
            <a
              key={s.anchor}
              href={`#${s.anchor}`}
              className={cx(css.item, s.level === 3 && css.l3, i === reading.at ? css.on : i < reading.at && css.read)}
              aria-current={i === reading.at ? "location" : undefined}
              tabIndex={hidden ? -1 : undefined}
              onClick={(e) => {
                if (e.metaKey || e.ctrlKey || e.shiftKey) return;
                e.preventDefault();
                go(s.anchor);
              }}
            >
              {s.text}
            </a>
          ))}
        </nav>
      ) : (
        <p className={css.empty}>No sections.</p>
      )}
    </aside>
  );
}
