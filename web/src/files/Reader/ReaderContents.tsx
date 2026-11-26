import { useRef } from "react";
import { Button } from "../../ui/Button";
import { cx } from "../../ui/cx";
import { Icon } from "../../ui/Icon";
import { Keys } from "../../ui/Keys";
import type { Section } from "../reading";
import css from "./ReaderContents.module.css";
import { useClickAway } from "./useClickAway";
import { usePlaceUnder } from "./usePlaceUnder";

// ReaderContents is the whole contents, opened from the pill or the
// Contents button (anchor: under it): each section, the one you're in
// marked, and its link to copy. A click anywhere else closes it, as t and
// esc do.
export function ReaderContents({ title, sections, at, anchor, go, copy, close }: { title: string; sections: Section[]; at: number; anchor: () => HTMLElement | null; go: (anchor: string) => void; copy: () => void; close: () => void }) {
  const box = useRef<HTMLDivElement>(null);
  const pos = usePlaceUnder(box, anchor);
  useClickAway(box, "[data-contents-toggle]", close);
  return (
    <div className={css.menu} ref={box} role="dialog" aria-label="Contents" style={pos ? { top: pos.top, left: pos.left } : { visibility: "hidden" }}>
      <div className={css.head}>
        <Icon name="list" size={12} />
        <b>{title}</b>
        <Button variant="link" className={css.action} type="button" onClick={copy} title="Copy link">
          Copy link <Icon name="link" size={12} />
        </Button>
      </div>
      <nav className={css.list}>
        {sections.length > 0 ? (
          sections.map((s, i) => (
            <button type="button" key={s.anchor} className={cx(css.item, s.level === 3 && css.l3, i === at && css.on)} aria-current={i === at ? "location" : undefined} onClick={() => go(s.anchor)}>
              {s.text}
            </button>
          ))
        ) : (
          <p className={css.empty}>No sections.</p>
        )}
      </nav>
      <div className={css.foot}>
        <Keys k="t" /> or <Keys k="esc" /> to close
      </div>
    </div>
  );
}
