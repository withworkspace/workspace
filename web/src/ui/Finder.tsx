import { createContext, type ReactNode, useContext, useEffect, useId, useRef, useState } from "react";
import { cx } from "./cx";
import css from "./Finder.module.css";
import { Icon } from "./Icon";
import { Spinner } from "./Spinner";
import { Input } from "./Input";
import { Modal } from "./Modal";

// What a row needs of its finder: where the cursor is, and what moves it
// or picks.
const Rows = createContext<{ id: string; at: number; over: (i: number) => void; pick: (i: number) => void } | undefined>(undefined);

// Finder is a text box over a list, in a Modal: what's typed says what
// the list holds (the finder's own to work out, as value changes), and
// one of it is picked. The cursor is on the first row from the start, and
// again whenever what's typed changes; ↓ and ↑ (or Ctrl+N and Ctrl+P)
// move it, as far as each end, and the pointer moving over a row puts it
// there; ⏎ or a press picks. The focus stays in the text box throughout:
// the text box says which row the cursor's on, as a combobox does.
//
// Its rows are FinderRows, each saying where it comes (index); count is
// how many there are. FinderNone is what it says with none. busy shows
// it's still looking, and after is what's under the list.
//
// It's driven by hand, not react-aria's autocomplete: that shows no
// cursor until a key's pressed, and the pointer doesn't move it.
//
// Find itself (shell/Palette.tsx) is more than this, but draws its box
// and its text box's row as this does: finderBox and finderInput.
export function Finder({
  value,
  onChange,
  onPick,
  onClose,
  count,
  placeholder,
  busy,
  className,
  listClassName,
  after,
  "aria-label": label,
  children,
}: {
  value: string;
  onChange: (value: string) => void;
  onPick: (index: number) => void;
  onClose: () => void;
  count: number;
  placeholder?: string;
  busy?: boolean;
  className?: string;
  listClassName?: string;
  after?: ReactNode;
  "aria-label": string;
  children: ReactNode; // FinderRows, or a FinderNone
}) {
  const id = useId();
  const list = useRef<HTMLUListElement>(null);
  const [cursor, setCursor] = useState(0);
  const at = Math.min(cursor, Math.max(0, count - 1));
  const move = (d: number) => setCursor(Math.max(0, Math.min(at + d, count - 1)));
  useEffect(() => {
    list.current?.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: "nearest" });
  }, [at]);
  return (
    <Modal onClose={onClose} overlayClassName={css.overlay} className={cx(css.box, className)} aria-label={label}>
      <div className={css.input}>
        <Icon name="search" size={16} />
        <Input
          autoFocus
          value={value}
          placeholder={placeholder}
          aria-label={label}
          role="combobox"
          aria-expanded="true"
          aria-controls={`${id}-list`}
          aria-activedescendant={count ? `${id}-${at}` : undefined}
          spellCheck={false}
          autoComplete="off"
          onChange={(e) => {
            onChange(e.target.value);
            setCursor(0);
          }}
          onKeyDown={(e) => {
            if (e.key === "ArrowDown" || (e.ctrlKey && e.key === "n")) {
              e.preventDefault();
              move(1);
            } else if (e.key === "ArrowUp" || (e.ctrlKey && e.key === "p")) {
              e.preventDefault();
              move(-1);
            } else if (e.key === "Enter" && count) {
              e.preventDefault();
              onPick(at);
            }
          }}
        />
        {busy && <Spinner size={14} />}
      </div>
      <ul className={cx(css.list, listClassName)} id={`${id}-list`} ref={list} role="listbox" aria-label={label}>
        <Rows.Provider value={{ id, at, over: setCursor, pick: onPick }}>{children}</Rows.Provider>
      </ul>
      {after}
    </Modal>
  );
}

// FinderRow is one of a Finder's rows: where it comes in the list, and
// what it shows.
export function FinderRow({ index, className, children }: { index: number; className?: string; children: ReactNode }) {
  const rows = useContext(Rows);
  if (!rows) throw new Error("a FinderRow is a Finder's");
  const on = index === rows.at;
  return (
    <li id={`${rows.id}-${index}`} role="option" aria-selected={on} className={cx(css.row, className)} onMouseMove={() => !on && rows.over(index)} onClick={() => rows.pick(index)}>
      {children}
    </li>
  );
}

// FinderNone is what a Finder says with nothing in its list.
export function FinderNone({ className, children }: { className?: string; children: ReactNode }) {
  return (
    <li role="presentation" className={cx(css.none, className)}>
      {children}
    </li>
  );
}

// A finder's box, and the row its text box is in, for Find to draw its
// own as.
export const finderOverlay = css.overlay;
export const finderBox = css.box;
export const finderInput = css.input;
