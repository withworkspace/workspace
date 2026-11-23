import { Fragment, type KeyboardEvent, type RefObject, useEffect, useMemo, useRef, useState } from "react";
import { Crumbs, CrumbSep } from "../ui/Crumbs";
import { Link } from "react-router";
import { type DocIn, GROUND, getSpace, getTree, type Space, type SpaceDoc, type SpaceRef, worktreeAPI } from "../api";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import { usePlaceUnder, useClickAway } from "./Contents";
import { fileShelves, filterItems, type Here, holds, type Shelf, type ShelfItem, type Shelves, type Siblings, spaceShelves } from "./shelves";
import { Input } from "../ui/Input";

// Getting around the reader (#123): its crumbs are its path, each step of
// it a menu of what's there (FolderMenu), and the documents beside it are
// at its foot (Pager), as [ and ] go to them. shelves.ts works out where it
// is; this loads that and draws it.

// What the shelves are read from: a space's documents, or a worktree's
// files. Kept, so going from one document to the next draws its path at
// once; read again behind it when it's older than this.
type Source = { space: Space; docs: SpaceDoc[] } | { files: string[] };
const kept = new Map<string, { at: number; source: Source }>();
const fresh = 30_000;

// useShelves is the shelves a document is read among: its space's (a
// space's document on its default branch), else its worktree's markdown.
// None until they're read, or if they can't be.
export function useShelves(where: DocIn | undefined, space: SpaceRef | undefined, here: Here | undefined): Shelves | undefined {
  const key = !where ? undefined : space ? `space:${space.repo}` : `tree:${worktreeAPI(where.repo, where.worktree)}`;
  const [source, setSource] = useState<Source | undefined>(() => (key ? kept.get(key)?.source : undefined));
  useEffect(() => {
    if (!key || !where) return setSource(undefined);
    const had = kept.get(key);
    setSource(had?.source);
    if (had && Date.now() - had.at < fresh) return;
    let stale = false;
    const read: Promise<Source> = space ? getSpace(space.repo).then((r) => ({ space: r.space, docs: r.docs })) : getTree(worktreeAPI(where.repo, where.worktree)).then((t) => (t.unread ? Promise.reject(new Error("still being read")) : { files: t.files.filter((f) => f.status !== "deleted").map((f) => f.path) }));
    read.then(
      (s) => {
        kept.set(key, { at: Date.now(), source: s });
        if (!stale) setSource(s);
      },
      () => {}, // its path is then just its name: nothing to say
    );
    return () => {
      stale = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);
  const label = where ? `${where.repo}/${where.worktree === GROUND ? "ground" : where.worktree}` : "";
  return useMemo(() => {
    if (!source) return undefined;
    return "space" in source ? spaceShelves(source.space, source.docs, here) : fileShelves(label, source.files, here);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [source, label, here?.path, here?.title, here?.kind]);
}

// PathCrumbs is the reader's path: where it's from (Documents, or Files),
// each shelf it's on as a menu of what's there, and its own title.
export function PathCrumbs({ from, on, title, open, toggle, nav }: { from: { to: string; label: string }; on: Shelf[]; title: string; open?: string; toggle: (id: string) => void; nav: RefObject<HTMLElement | null> }) {
  return (
    <Crumbs className="path-crumbs" ref={nav}>
      <Link to={from.to} className="crumb">
        {from.label}
      </Link>
      <CrumbSep className="path-sep">/</CrumbSep>
      {on.map((s) => (
        <Fragment key={s.id}>
          <button
            type="button"
            className={`crumb${open === s.id ? " open" : ""}`}
            data-folder-crumb={s.id}
            aria-haspopup="dialog"
            aria-expanded={open === s.id}
            title={`What's in ${s.name}`}
            onClick={() => toggle(s.id)}
          >
            {s.label}
            <Icon name="chevron-down" size={11} />
          </button>
          <CrumbSep className="path-sep">/</CrumbSep>
        </Fragment>
      ))}
      <span className="crumb here" aria-current="page">
        {title}
      </span>
    </Crumbs>
  );
}

// FolderMenu is what's on one of the shelves in the path: filter it, step
// onto a shelf on it, go up, or open it where it's listed in full (Open in
// the space, or in Files). It starts on the document, or the shelf it's
// on. A new shelf is a new menu (keyed by it), nothing typed.
export function FolderMenu({
  shelves,
  id,
  here,
  on,
  anchor,
  column,
  openIn,
  go,
  step,
  close,
}: {
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
}) {
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
    box.current?.querySelector(".folder-menu-item.sel")?.scrollIntoView({ block: "nearest" });
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
    <div className="folder-menu" ref={box} role="dialog" aria-label={`What's in ${shelf.name}`} onKeyDown={keys} style={pos ? { top: pos.top, left: pos.left } : { visibility: "hidden" }}>
      <div className="folder-menu-head">
        {shelf.up !== undefined && (
          <button type="button" className="folder-up" onClick={up} title="Up a folder (u, or ⌫ with nothing typed)" aria-label="Up a folder">
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
        className="folder-menu-filter"
        placeholder={`Filter ${shelf.label}…`}
        aria-label={`Filter ${shelf.name}`}
        value={typed}
        onChange={(e) => {
          setTyped(e.target.value);
          setSel(0);
        }}
      />
      <div className="folder-menu-list" role="listbox" aria-label={shelf.name}>
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
                className={`folder-menu-item${n === at ? " sel" : ""}${isHere ? " here" : ""}`}
                onMouseMove={() => n !== at && setSel(n)}
                onClick={() => choose(i)}
              >
                <Icon name={i.shelf === undefined ? "file" : i.shelf.startsWith("kind:") ? "list" : "folder"} size={12} />
                <span className="folder-menu-title">{i.title}</span>
                <span className="folder-menu-note">
                  {isHere ? "you're here" : i.note}
                  {i.shelf !== undefined && <Icon name="chevron-right" size={12} />}
                </span>
              </button>
            );
          })
        ) : (
          <p className="contents-empty">{typed ? "Nothing matches." : "Nothing here."}</p>
        )}
      </div>
      <div className="folder-menu-foot">
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

// Pager is the documents either side of it, and where it is among those
// kept with it (17 of 28 in its folder), at its foot: [ and ] go there.
export function Pager({ sib, to }: { sib: Siblings; to: (path: string) => string }) {
  const where = sib.shelf.name;
  return (
    <nav className="doc-pager" aria-label={`In ${where}`}>
      {sib.prev?.path ? (
        <Link className="prev" to={to(sib.prev.path)} rel="prev">
          <small>
            <Icon name="arrow-left" size={12} /> Previous in {where} <Keys k="[" />
          </small>
          <b>{sib.prev.title}</b>
        </Link>
      ) : (
        <span className="none" />
      )}
      <span className="pos">
        {sib.at} of {sib.of}
        <br />
        in {where}
      </span>
      {sib.next?.path ? (
        <Link className="next" to={to(sib.next.path)} rel="next">
          <small>
            <Keys k="]" /> Next in {where} <Icon name="arrow-right" size={12} />
          </small>
          <b>{sib.next.title}</b>
        </Link>
      ) : (
        <span className="none" />
      )}
    </nav>
  );
}
