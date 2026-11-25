import { type KeyboardEvent, useEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router";
import { cx } from "../../ui/cx";
import { Icon } from "../../ui/Icon";
import { Input } from "../../ui/Input";
import { Keys } from "../../ui/Keys";
import { filterItems, holds, type Shelf, type ShelfItem, type Shelves } from "../shelves";
import css from "./ReaderFolder.module.css";
import { useClickAway } from "./useClickAway";
import { usePlaceUnder } from "./usePlaceUnder";

type ReaderFolderProps = {
  shelves: Shelves;
  id: string;
  here: string; // the document's path
  on: Shelf[]; // the shelves it's kept on, top first
  anchor: () => HTMLElement | null;
  column: () => HTMLElement | null;
  openIn: (shelf: Shelf) => { to: string; label: string };
  go: (path: string) => void;
  step: (id: string) => void;
  close: () => void;
};

// ReaderFolder is what's on one of the shelves in the reader's path:
// filter it, step onto a shelf on it, go up, or open it where it's listed
// in full (Open in the space, or in Files). It starts on the document, or
// the shelf it's on. A new shelf is a new menu (keyed by it), nothing
// typed.
export function ReaderFolder({ shelves, id, here, on, anchor, column, openIn, go, step, close }: ReaderFolderProps) {
  const shelf = shelves.shelves.get(id);
  const [typed, setTyped] = useState("");
  const items = useMemo(() => filterItems(shelf?.items ?? [], typed), [shelf, typed]);
  const [sel, setSel] = useState(() => Math.max(0, shelf?.items.findIndex((i) => holds(i, here, on)) ?? 0));
  const at = Math.min(sel, items.length - 1);
  const box = useRef<HTMLDivElement>(null);
  const filter = useRef<HTMLInputElement>(null);
  const pos = usePlaceUnder(box, anchor, column);
  useClickAway(box, "[data-folder-crumb]", close);
  // Typing filters it from the start: once it's placed (hidden till then,
  // it can't take the focus).
  const placed = !!pos;
  useEffect(() => {
    if (placed) filter.current?.focus({ preventScroll: true });
  }, [placed]);
  useEffect(() => {
    box.current?.querySelector('[role="option"][aria-selected="true"]')?.scrollIntoView({ block: "nearest" });
  }, [at, pos]);
  if (!shelf) return null;
  const up = () => shelf.up !== undefined && step(shelf.up);
  const choose = (i: ShelfItem | undefined) => {
    if (i?.shelf !== undefined) step(i.shelf);
    else if (i?.path === here) close();
    else if (i?.path) go(i.path);
  };
  const keys = (e: KeyboardEvent) => {
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    const empty = typed === "";
    if (e.key === "ArrowDown") setSel(Math.min(items.length - 1, at + 1));
    else if (e.key === "ArrowUp") setSel(Math.max(0, at - 1));
    else if (e.key === "Enter" || (e.key === "ArrowRight" && items[at]?.shelf !== undefined)) choose(items[at]);
    else if (empty && (e.key === "Backspace" || e.key === "ArrowLeft" || e.key === "u")) up();
    else if (e.key === "Escape") close();
    else return;
    e.preventDefault();
    e.stopPropagation();
  };
  const where = openIn(shelf);
  return (
    <div className={css.menu} ref={box} role="dialog" aria-label={`What's in ${shelf.name}`} onKeyDown={keys} style={pos ? { top: pos.top, left: pos.left } : { visibility: "hidden" }}>
      <div className={css.head}>
        {shelf.up !== undefined && (
          <button type="button" className={css.up} onClick={up} title="Up a folder (u, or ⌫ with nothing typed)" aria-label="Up a folder">
            <Icon name="arrow-up" size={12} />
          </button>
        )}
        <Icon name={shelf.kind ? "list" : "folder"} size={12} className="dim" />
        <b>{shelf.name}</b>
        <Link to={where.to} onClick={close}>
          {where.label} <Icon name="arrow-right" size={12} />
        </Link>
      </div>
      <Input
        ref={filter}
        className={css.filter}
        placeholder={`Filter ${shelf.label}…`}
        aria-label={`Filter ${shelf.name}`}
        value={typed}
        onChange={(e) => {
          setTyped(e.target.value);
          setSel(0);
        }}
      />
      <div className={css.list} role="listbox" aria-label={shelf.name}>
        {items.length > 0 ? (
          items.map((i, n) => {
            const isHere = i.path === here;
            return (
              <button
                type="button"
                key={i.shelf ?? i.path}
                role="option"
                aria-selected={n === at}
                aria-current={isHere ? "page" : undefined}
                className={cx(css.item, n === at && css.sel, isHere && css.here)}
                onMouseMove={() => n !== at && setSel(n)}
                onClick={() => choose(i)}
              >
                <Icon name={i.shelf === undefined ? "file" : i.shelf.startsWith("kind:") ? "list" : "folder"} size={12} />
                <span className={css.title}>{i.title}</span>
                <span className={css.note}>
                  {isHere ? "you're here" : i.note}
                  {i.shelf !== undefined && <Icon name="chevron-right" size={12} />}
                </span>
              </button>
            );
          })
        ) : (
          <p className={css.empty}>{typed ? "Nothing matches." : "Nothing here."}</p>
        )}
      </div>
      <div className={css.foot}>
        <span>
          <Keys k="↑" />
          <Keys k="↓" /> move
        </span>
        <span>
          <Keys k="⏎" /> open
        </span>
        <span>
          <Keys k="u" /> up a folder
        </span>
        <span>
          <Keys k="esc" />
        </span>
      </div>
    </div>
  );
}
