import { Button } from "../../ui/Button";
import { cx } from "../../ui/cx";
import { Icon } from "../../ui/Icon";
import { Keys } from "../../ui/Keys";
import { Menu, MenuItem } from "../../ui/Menu";
import css from "./FilesPanes.module.css";
import type { PaneChoice, Panes } from "./panes";

// FilesPanes picks how many panes show, from the worktree's row: at its
// start, before where it is. Each choice is drawn as the window it makes,
// the one it's on ticked; p switches them, with the menu open too.
export function FilesPanes({ now, choices, onPick }: { now: PaneChoice; choices: PaneChoice[]; onPick: (n: Panes) => void }) {
  return (
    <Menu
      className={css.popover}
      aria-label="Panes"
      onAction={(k) => onPick(Number(k) as Panes)}
      pageKeys
      trigger={
        <Button variant="ghost" square className={css.button} aria-label={`Panes: ${now.label}`} title={`Panes: ${now.label}`} keys="p">
          <FilesPanesGlyph n={now.n} />
        </Button>
      }
      foot={
        <div className={cx("dim", css.keys)}>
          Switch with <Keys k="p" />
        </div>
      }
    >
      {choices.map((c) => (
        <MenuItem key={c.n} id={String(c.n)} textValue={c.label} className={css.option}>
          <FilesPanesGlyph n={c.n} className={css.glyph} />
          <span className={css.words}>
            <span>{c.label}</span>
            <span className="dim">{c.sub}</span>
          </span>
          {c.n === now.n && <Icon name="check" size={13} className={css.check} />}
        </MenuItem>
      ))}
    </Menu>
  );
}

// FilesPanesGlyph draws n panes as a window split in n, the last (the
// file) widest. It's drawn as an Icon is.
export function FilesPanesGlyph({ n, className }: { n: Panes; className?: string }) {
  return (
    <svg className={cx("icon", className)} width={15} height={15} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <rect x={3} y={4} width={18} height={16} rx={2} />
      {n >= 2 && <path d="M9 4v16" />}
      {n === 3 && <path d="M6 4v16" />}
    </svg>
  );
}
