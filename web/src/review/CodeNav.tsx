import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { Link, useNavigate } from "react-router";
import { getBlob, type DiffFile } from "../api";
import { useKeys } from "../keys";
import { Icon } from "../ui/Icon";
import { toast } from "../ui/kit";
import { Keys } from "../ui/Keys";
import { escapeHTML, highlightLines, languageOf } from "./highlight";
import type { FileState, Row } from "./model";
import { answerWords, caretAt, gapOf, getDefinition, getNames, getReferences, groupUses, nameAt, rangeOf, rowOf, textOffset, type NavAnswer, type NavRef, type NavReferences, type NavTarget, type Place, type Side } from "./nav";
import { Button, buttonClass } from "../ui/Button";

// Following names in a review: hold ⌘ and a name under the pointer is a
// link; rest on it for its card, click it to go to its definition (in the
// diff if it's there, in a panel beside it if not). d does it from the
// keyboard, b goes back. The explorer follows names too, in a file that's
// not a diff: a definition opens there (NavProps.open).

// A code cell says where its line is: data-rev, data-path, data-line and
// data-side, read here.
export function cellPlace(el: Element): { rev: string; path: string; line: number; side: Side; inPanel: boolean } | undefined {
  const code = el.closest<HTMLElement>("code[data-rev]");
  if (!code) return undefined;
  const { rev, path, line, side } = code.dataset;
  if (!rev || !path || !line) return undefined;
  return { rev, path, line: Number(line), side: side === "old" ? "old" : "new", inPanel: !!code.closest(".nav-panel") };
}

// Where's somewhere to go back to: a row of the diff, or the panel open at
// a line.
type Where = { row: string } | { panel: PanelAt };

// PanelAt is what the panel shows: a file at a line, or who uses a
// definition.
interface PanelAt {
  rev: string;
  path: string;
  line: number;
  side: Side;
  how?: string;
  uses?: NavReferences;
}

// At is a name's place, and what part it plays there.
type At = Place & { side: Side; inPanel: boolean; part?: string };

interface Card {
  answer?: NavAnswer; // undefined while it's asked
  place: At;
  rect: DOMRect;
  pinned: boolean; // opened by a click or d: stays until closed
  pick: number;
}

interface Picking {
  side: Side;
  rev: string;
  path: string;
  line: number;
  names: { from: number; to: number; text: string; part: string }[];
  pick: number;
  rect: DOMRect;
}

export interface NavProps {
  api: string;
  diff?: { base: string; head: string };
  files: DiffFile[];
  rows: Row[][];
  states: FileState[];
  pane: React.RefObject<HTMLDivElement | null>;
  cursorRow?: Row;
  expand: (file: number, gap: number, n: number) => Promise<void>;
  goToRow: (key: string) => void;
  browse?: (path: string, line: number) => string | undefined;
  // In a file that's not a diff (the explorer, whose diff is one commit
  // both sides): open goes to a file at a line there, back goes back from
  // it (false when there's nowhere), line is the line d asks about, and
  // sideWords says what the commit is.
  open?: (path: string, line: number) => void;
  back?: () => boolean;
  line?: { path: string; line: number; text: string };
  sideWords?: (side: Side) => string;
}

// reviewSides is what a review's commits are.
const reviewSides = (side: Side) => (side === "old" ? "before the change" : "with the change");

// The most lines a gap is opened by to show a definition in it; past that
// it's shown in the panel.
const openUpTo = 200;

const supportsHighlights = typeof CSS !== "undefined" && "highlights" in CSS;

// told are the files that have said why they've no names: once each.
const told = new Set<string>();

function setLink(range: Range | undefined) {
  if (!supportsHighlights) return;
  const reg = (CSS as unknown as { highlights: Map<string, unknown> }).highlights;
  if (range) reg.set("nav-link", new (window as unknown as { Highlight: new (r: Range) => unknown }).Highlight(range));
  else reg.delete("nav-link");
}

// useCodeNav is navigation for a review: what to draw (the card and the
// panel), and its keys.
export function useCodeNav(p: NavProps) {
  const [card, setCard] = useState<Card>();
  const [panel, setPanel] = useState<PanelAt>();
  const [picking, setPicking] = useState<Picking>();
  const back = useRef<Where[]>([]);
  const forward = useRef<Where[]>([]);
  const props = useRef(p);
  props.current = p;
  // A target waiting for a gap to open before it's gone to.
  const [pending, setPending] = useState<{ file: number; side: Side; line: number }>();

  const panelNow = useRef(panel);
  panelNow.current = panel;
  const where = (): Where | undefined => {
    if (panelNow.current) return { panel: panelNow.current };
    const key = props.current.cursorRow?.key;
    return key ? { row: key } : undefined;
  };
  const remember = () => {
    const w = where();
    if (w) back.current = [...back.current.slice(-19), w];
    forward.current = [];
  };
  const restore = (w: Where) => {
    if ("row" in w) {
      setPanel(undefined);
      props.current.goToRow(w.row);
    } else setPanel(w.panel);
  };

  // go is to a definition, in the commit the name was read at.
  const go = useCallback(async (t: NavTarget, side: Side, how: string | undefined, inPanel: boolean) => {
    const { files, rows, states, expand, goToRow, open } = props.current;
    setCard(undefined);
    if (open) {
      setPanel(undefined);
      open(t.path, t.line);
      return;
    }
    remember();
    if (!inPanel) {
      const i = files.findIndex((f) => (side === "old" ? (f.oldPath ?? f.path) : f.path) === t.path);
      if (i >= 0) {
        const key = rowOf(rows[i], side, t.line);
        if (key) {
          setPanel(undefined);
          goToRow(key);
          return;
        }
        const g = gapOf(files[i], states[i].blob, side, t.line);
        if (g && g.size !== undefined && g.size <= openUpTo) {
          setPanel(undefined);
          await expand(i, g.gap, 0);
          setPending({ file: i, side, line: t.line });
          return;
        }
      }
    }
    const rev = side === "new" ? props.current.diff?.head : props.current.diff?.base;
    if (rev) setPanel({ rev: inPanel ? (panelNow.current?.rev ?? rev) : rev, path: t.path, line: t.line, side, how });
  }, []);

  // Once a gap's opened, the line it held is a row to go to.
  useEffect(() => {
    if (!pending) return;
    const key = rowOf(p.rows[pending.file] ?? [], pending.side, pending.line);
    if (key) {
      setPending(undefined);
      p.goToRow(key);
    }
  }, [pending, p.rows]);

  // usesOf opens who uses the definition at a place, in the panel.
  const usesOf = (place: At) => {
    getReferences(props.current.api, place).then(
      (refs) => {
        if (!refs.def) return toast(`${refs.name} isn't defined in the repo`);
        setCard(undefined);
        remember();
        setPanel({ rev: place.rev, path: refs.def.path, line: refs.def.line, side: place.side, uses: refs });
      },
      (e: Error) => toast(e.message, { tone: "danger" }),
    );
  };

  // follow is what a click (or d) does with an answer.
  const follow = useCallback(
    (a: NavAnswer, place: At, rect: DOMRect) => {
      if (a.answer === "external" || a.answer === "builtin") {
        if (a.external?.url) window.open(a.external.url, "_blank", "noopener");
        else toast(`${a.name} is ${a.answer === "builtin" ? "built in" : `from ${a.external?.module ?? "a dependency"}`}`);
        return;
      }
      if (a.answer === "unresolved" || a.targets.length === 0) {
        toast(`There's no definition of ${a.name} here`);
        return;
      }
      if (a.targets.length === 1 && a.answer === "resolved") {
        go(a.targets[0], place.side, a.how, place.inPanel);
        return;
      }
      setCard({ answer: a, place, rect, pinned: true, pick: 0 });
    },
    [go],
  );

  // ask is the definition of the name at a place, and its card. A
  // definition's own name is followed to who uses it.
  const ask = (place: At, rect: DOMRect, then: "card" | "follow") => {
    if (then === "follow" && place.part === "d") return usesOf(place);
    getDefinition(props.current.api, place).then(
      (a) => {
        if (then === "follow") follow(a, place, rect);
        else setCard((c) => (c && !c.pinned && c.place === place ? { ...c, answer: a } : c));
      },
      (e: Error) => then === "follow" && toast(e.message, { tone: "danger" }),
    );
  };

  // Pointing, with ⌘ held: underline the name under the pointer, and
  // after a moment, its card.
  const hover = useRef<{ place: At; rect: DOMRect } | undefined>(undefined);
  const rest = useRef<number>(undefined);
  const mod = useRef(false);
  useEffect(() => {
    const clear = () => {
      setLink(undefined);
      hover.current = undefined;
      window.clearTimeout(rest.current);
      document.body.classList.remove("nav-pointing");
      setCard((c) => (c?.pinned ? c : undefined));
    };
    let seq = 0;
    const point = (x: number, y: number) => {
      const caret = caretAt(x, y);
      const el = caret && (caret.node.nodeType === Node.TEXT_NODE ? caret.node.parentElement : (caret.node as Element));
      const at = el && cellPlace(el);
      const code = el?.closest("code[data-rev]");
      if (!caret || !at || !code) return clear();
      const col = textOffset(code, caret.node, caret.offset);
      if (col === undefined) return clear();
      const mine = ++seq;
      getNames(props.current.api, at.rev, at.path).then((names) => {
        if (mine !== seq || !mod.current) return;
        if (names.why && !told.has(at.path)) {
          told.add(at.path);
          toast(names.why);
        }
        const n = nameAt(names, at.line, col);
        if (!n) return clear();
        const range = rangeOf(code, n[0], n[1]);
        if (!range) return clear();
        const place: At = { rev: at.rev, path: at.path, line: at.line, col: n[0], side: at.side, inPanel: at.inPanel, part: n[2] };
        if (hover.current && hover.current.place.path === place.path && hover.current.place.line === place.line && hover.current.place.col === place.col && hover.current.place.rev === place.rev) return;
        const rect = range.getBoundingClientRect();
        hover.current = { place, rect };
        setLink(range);
        document.body.classList.add("nav-pointing");
        window.clearTimeout(rest.current);
        setCard((c) => (c?.pinned ? c : undefined));
        rest.current = window.setTimeout(() => {
          if (hover.current?.place !== place) return;
          setCard({ place, rect, pinned: false, pick: 0 });
          ask(place, rect, "card");
        }, 350);
      }, clear);
    };
    let lastX = 0;
    let lastY = 0;
    const onMove = (e: MouseEvent) => {
      lastX = e.clientX;
      lastY = e.clientY;
      mod.current = e.metaKey || e.ctrlKey;
      if (mod.current) point(e.clientX, e.clientY);
      else if (hover.current) clear();
    };
    const onKey = (e: KeyboardEvent) => {
      const held = e.metaKey || e.ctrlKey;
      if (held === mod.current) return;
      mod.current = held;
      if (held) point(lastX, lastY);
      else clear();
    };
    const onDown = (e: MouseEvent) => {
      if (!(e.metaKey || e.ctrlKey) || e.button !== 0 || !hover.current) return;
      const target = e.target as Element;
      if (!target.closest("code[data-rev]")) return;
      // The name's link, not the row's click or a selection.
      e.preventDefault();
      e.stopPropagation();
      const { place, rect } = hover.current;
      window.clearTimeout(rest.current);
      ask(place, rect, "follow");
    };
    const onClick = (e: MouseEvent) => {
      if ((e.metaKey || e.ctrlKey) && hover.current && (e.target as Element).closest("code[data-rev]")) {
        e.preventDefault();
        e.stopPropagation();
      }
    };
    const onBlur = () => {
      mod.current = false;
      clear();
    };
    window.addEventListener("mousemove", onMove);
    window.addEventListener("keydown", onKey);
    window.addEventListener("keyup", onKey);
    window.addEventListener("mousedown", onDown, true);
    window.addEventListener("click", onClick, true);
    window.addEventListener("blur", onBlur);
    return () => {
      clear();
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("keydown", onKey);
      window.removeEventListener("keyup", onKey);
      window.removeEventListener("mousedown", onDown, true);
      window.removeEventListener("click", onClick, true);
      window.removeEventListener("blur", onBlur);
    };
  }, []);

  // The cursor's line, for d: a diff's row, or the line picked in a file.
  const cursorLine = (): { side: Side; rev: string; path: string; num: number; text: string; el: HTMLElement | null } | undefined => {
    const { cursorRow, files, diff, line } = props.current;
    if (!diff) return undefined;
    if (line) return { side: "new", rev: diff.head, path: line.path, num: line.line, text: line.text, el: document.querySelector<HTMLElement>(`[data-line="${line.line}"]`) };
    if (cursorRow?.type !== "line") return undefined;
    const f = files[cursorRow.file];
    const useOld = !cursorRow.right; // a removed line
    const cell = useOld ? cursorRow.left : cursorRow.right;
    if (!cell) return undefined;
    const side: Side = useOld ? "old" : "new";
    return {
      side,
      rev: side === "new" ? diff.head : diff.base,
      path: side === "old" ? (f.oldPath ?? f.path) : f.path,
      num: cell.num,
      text: cell.text,
      el: document.getElementById(`row-${cursorRow.key}`),
    };
  };

  // d: the names on the cursor's line, to pick one of.
  const pickOnRow = async () => {
    const at = cursorLine();
    if (!at) {
      if (props.current.line === undefined && props.current.open) toast("Pick a line first: click its number");
      return;
    }
    const { side, rev, path } = at;
    const cell = { num: at.num, text: at.text };
    let names;
    try {
      names = await getNames(props.current.api, rev, path);
    } catch (e) {
      toast((e as Error).message, { tone: "danger" });
      return;
    }
    if (names.why) return toast(names.why);
    const on = names.lines[String(cell.num)] ?? [];
    if (!on.length) return toast("There are no names on this line");
    const rect = at.el?.getBoundingClientRect() ?? new DOMRect(0, 0, 0, 0);
    const list = on.map(([from, to, part]) => ({ from, to, part, text: cell.text.slice(from, to) }));
    if (list.length === 1) {
      const place: At = { rev, path, line: cell.num, col: list[0].from, side, inPanel: false, part: list[0].part };
      return ask(place, rect, "follow");
    }
    setPicking({ side, rev, path, line: cell.num, names: list, pick: 0, rect });
  };

  const keys = {
    d: pickOnRow,
    b: () => {
      const w = back.current.pop();
      if (!w) {
        // In a file: the panel closes first, then back to where it was.
        if (panelNow.current) return setPanel(undefined);
        if (props.current.back?.()) return;
        return toast("Nowhere to go back to");
      }
      const now = where();
      if (now) forward.current.push(now);
      restore(w);
    },
    B: () => {
      const w = forward.current.pop();
      if (!w) return;
      const now = where();
      if (now) back.current.push(now);
      restore(w);
    },
  };

  const cardEl = card && (
    <NavCard
      card={card}
      onClose={() => setCard(undefined)}
      onGo={(t) => (card.place.part === "d" ? usesOf(card.place) : go(t, card.place.side, card.answer?.how, card.place.inPanel))}
      onFollow={() => (card.place.part === "d" ? usesOf(card.place) : card.answer && follow(card.answer, card.place, card.rect))}
    />
  );
  const pickEl = picking && (
    <NamePicker
      picking={picking}
      onClose={() => setPicking(undefined)}
      onPick={(n) => {
        setPicking(undefined);
        ask({ rev: picking.rev, path: picking.path, line: picking.line, col: n.from, side: picking.side, inPanel: false, part: n.part }, picking.rect, "follow");
      }}
    />
  );
  // What the change touches, for who uses it: its files, and its changed
  // lines on a side.
  const fileOf = (path: string, side: Side) => p.files.findIndex((f) => (side === "old" ? (f.oldPath ?? f.path) : f.path) === path);
  const changed = (path: string, side: Side, line: number) => {
    const i = fileOf(path, side);
    return i >= 0 && p.rows[i].some((r) => r.type === "line" && (side === "new" ? r.right?.num === line && r.right.kind === "a" : r.left?.num === line && r.left.kind === "d"));
  };
  const panelEl =
    panel &&
    (panel.uses ? (
      <UsesPanel
        key={`uses:${panel.rev}:${panel.path}:${panel.line}`}
        at={panel}
        uses={panel.uses}
        inChange={(path) => fileOf(path, panel.side) >= 0}
        changed={(path, line) => changed(path, panel.side, line)}
        onGo={(u) => go({ path: u.path, line: u.line, col: u.col, endLine: u.line, kind: "use" }, panel.side, undefined, false)}
        sideWords={p.sideWords ?? reviewSides}
        onClose={() => setPanel(undefined)}
        onBack={keys.b}
      />
    ) : (
      <NavPanel
        key={`${panel.rev}:${panel.path}`}
        api={p.api}
        at={panel}
        browse={p.browse?.(panel.path, panel.line)}
        sideWords={p.sideWords ?? reviewSides}
        onClose={() => setPanel(undefined)}
        onBack={keys.b}
      />
    ));
  return { card: cardEl, picker: pickEl, panel: panelEl, keys };
}

// NavCard is a name's card: where it's defined and what it says, or the
// definitions it may be, to pick from.
function NavCard({ card, onClose, onGo, onFollow }: { card: Card; onClose: () => void; onGo: (t: NavTarget) => void; onFollow: () => void }) {
  const a = card.answer;
  const list = a && (a.answer === "candidates" || a.targets.length > 1) ? a.targets : undefined;
  const [pick, setPick] = useState(0);
  useKeys(
    card.pinned
      ? {
          Escape: onClose,
          j: () => setPick((i) => Math.min(i + 1, (list?.length ?? 1) - 1)),
          ArrowDown: () => setPick((i) => Math.min(i + 1, (list?.length ?? 1) - 1)),
          k: () => setPick((i) => Math.max(i - 1, 0)),
          ArrowUp: () => setPick((i) => Math.max(i - 1, 0)),
          Enter: () => (list ? onGo(list[pick]) : onFollow()),
        }
      : {},
    { modal: card.pinned },
  );
  const ref = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState<{ top: number; left: number }>();
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const h = el.offsetHeight;
    const w = el.offsetWidth;
    const below = card.rect.bottom + 6;
    const top = below + h > window.innerHeight - 8 ? Math.max(8, card.rect.top - h - 6) : below;
    setPos({ top, left: Math.max(8, Math.min(card.rect.left, window.innerWidth - w - 8)) });
  }, [card.rect, a]);
  const one = a && !list ? a.targets[0] : undefined;
  return (
    <div ref={ref} className={`nav-card${card.pinned ? " pinned" : ""}`} style={pos ?? { visibility: "hidden" }} onMouseDown={(e) => e.stopPropagation()}>
      {!a && <div className="dim">Looking…</div>}
      {one && (
        <button className="nav-card-target" onClick={() => onGo(one)}>
          {one.signature && <code className="nav-sig">{one.signature}</code>}
          {one.doc && <div className="nav-doc">{one.doc}</div>}
          <div className="nav-where mono dim">
            {one.path}:{one.line}
          </div>
        </button>
      )}
      {a && !one && !list && <div className="nav-sig">{a.external?.module ? `${a.external.module}${a.external.name ? `.${a.external.name}` : ""}` : a.name}</div>}
      {list && (
        <ul className="nav-list">
          {list.map((t, i) => (
            <li key={`${t.path}:${t.line}`} className={i === pick ? "picked" : ""} onMouseEnter={() => setPick(i)} onClick={() => onGo(t)}>
              <code className="nav-sig">{t.signature || `${t.kind} ${a!.name}`}</code>
              <span className="mono dim">
                {t.path}:{t.line}
              </span>
            </li>
          ))}
        </ul>
      )}
      {a && (
        <div className="nav-how dim">
          <span className={`nav-answer a-${a.answer}`}>{card.place.part === "d" ? "Defined here: click for who uses it" : answerWords(a)}</span>
          {a.answer === "candidates" && a.how && <span> · {a.how}</span>}
          {a.partial && <span title="It ran out of time or files to read, and may have missed some"> · partial</span>}
          {a.external?.url && (
            <a href={a.external.url} target="_blank" rel="noreferrer">
              Docs <Icon name="external" size={10} />
            </a>
          )}
          {card.pinned && <Keys k="esc" />}
        </div>
      )}
    </div>
  );
}

// NamePicker is d's list: the names on a line, to follow one.
function NamePicker({ picking, onClose, onPick }: { picking: Picking; onClose: () => void; onPick: (n: Picking["names"][number]) => void }) {
  const [pick, setPick] = useState(0);
  const n = picking.names.length;
  useKeys(
    {
      Escape: onClose,
      j: () => setPick((i) => Math.min(i + 1, n - 1)),
      ArrowDown: () => setPick((i) => Math.min(i + 1, n - 1)),
      k: () => setPick((i) => Math.max(i - 1, 0)),
      ArrowUp: () => setPick((i) => Math.max(i - 1, 0)),
      Enter: () => onPick(picking.names[pick]),
      d: () => onPick(picking.names[pick]),
    },
    { modal: true },
  );
  const top = Math.min(picking.rect.bottom + 4, window.innerHeight - 40 - n * 26);
  return (
    <div className="nav-card pinned nav-picker" style={{ top, left: Math.max(8, picking.rect.left + 120) }} onMouseDown={(e) => e.stopPropagation()}>
      <div className="dim nav-how">Go to the definition of</div>
      <ul className="nav-list">
        {picking.names.map((x, i) => (
          <li key={`${x.from}`} className={i === pick ? "picked" : ""} onMouseEnter={() => setPick(i)} onClick={() => onPick(x)}>
            <code>{x.text}</code>
            <span className="dim">{x.part === "m" ? "member" : x.part === "d" ? "defined here" : x.part === "i" ? "import" : ""}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}

// NavPanel is a file at a commit, beside the diff, at a definition. Names
// in it are links too, and following one stays in the panel.
function NavPanel({ api, at, browse, sideWords, onClose, onBack }: { api: string; at: PanelAt; browse?: string; sideWords: (side: Side) => string; onClose: () => void; onBack: () => void }) {
  const navigate = useNavigate();
  const [lines, setLines] = useState<string[]>();
  const [error, setError] = useState<string>();
  useEffect(() => {
    getBlob(api, at.rev, at.path).then(setLines, (e: Error) => setError(e.message));
  }, [api, at.rev, at.path]);
  const body = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = body.current?.querySelector<HTMLElement>(`[data-line="${at.line}"]`);
    if (!el) return;
    el.scrollIntoView({ block: "center" });
    el.classList.remove("nav-flash");
    void el.offsetWidth;
    el.classList.add("nav-flash");
  }, [lines, at.line]);
  const scroll = (by: number) => body.current?.scrollBy({ top: by * 20 });
  useKeys(
    {
      Escape: onClose,
      j: () => scroll(1),
      ArrowDown: () => scroll(1),
      k: () => scroll(-1),
      ArrowUp: () => scroll(-1),
      "mod+d": () => scroll(20),
      "mod+u": () => scroll(-20),
      b: onBack,
      o: () => browse && navigate(browse),
    },
    { modal: true },
  );
  const html = lines ? highlightLines(lines, languageOf(at.path)) : [];
  return (
    <aside className="nav-panel" aria-label={`${at.path} at ${at.rev.slice(0, 7)}`}>
      <header className="nav-panel-head">
        <span className="mono">
          {at.path}:{at.line}
        </span>
        <span className="dim">
          {sideWords(at.side)} · {at.rev.slice(0, 7)}
        </span>
        {at.how && <span className="dim nav-panel-how">{at.how}</span>}
        <span className="spacer-left" />
        {browse && (
          <Link to={browse} className={buttonClass({ size: "small" })} title="Open it in Files, as it is now (o)">
            Open <Keys k="o" />
          </Link>
        )}
        <Button variant="ghost" size="small" onClick={onBack} title="Back (b)">
          <Icon name="arrow-left" size={12} /> <Keys k="b" />
        </Button>
        <Button variant="ghost" size="small" onClick={onClose} title="Close (esc)">
          <Icon name="x" size={13} />
        </Button>
      </header>
      <div className="nav-panel-body lines" ref={body}>
        {error && <div className="banner">{error}</div>}
        {!lines && !error && <div className="dim nav-panel-loading">Reading {at.path}…</div>}
        {html.map((h, i) => (
          <div key={i} className="dl nav-line" data-line={i + 1}>
            <span className="ln">{i + 1}</span>
            <code data-rev={at.rev} data-path={at.path} data-line={i + 1} data-side={at.side} dangerouslySetInnerHTML={{ __html: h || escapeHTML("") }} />
          </div>
        ))}
      </div>
    </aside>
  );
}

// UsesPanel is who uses a definition: in this change first, then
// elsewhere, maybes folded under each.
function UsesPanel(props: {
  at: PanelAt;
  uses: NavReferences;
  inChange: (path: string) => boolean;
  changed: (path: string, line: number) => boolean;
  onGo: (u: NavRef) => void;
  sideWords: (side: Side) => string;
  onClose: () => void;
  onBack: () => void;
}) {
  const { at, uses, inChange, changed, onGo, sideWords, onClose, onBack } = props;
  const groups = groupUses(uses.uses, inChange, changed);
  const [open, setOpen] = useState<Record<string, boolean>>({});
  // The list as it's moved through: maybes only when opened.
  const flat = groups.flatMap((g) => [...g.uses, ...(open[g.title] ? g.maybes : [])]);
  const [pick, setPick] = useState(0);
  useKeys(
    {
      Escape: onClose,
      j: () => setPick((i) => Math.min(i + 1, flat.length - 1)),
      ArrowDown: () => setPick((i) => Math.min(i + 1, flat.length - 1)),
      k: () => setPick((i) => Math.max(i - 1, 0)),
      ArrowUp: () => setPick((i) => Math.max(i - 1, 0)),
      Enter: () => flat[pick] && onGo(flat[pick]),
      b: onBack,
    },
    { modal: true },
  );
  useEffect(() => {
    document.querySelector(".nav-uses li.picked")?.scrollIntoView({ block: "nearest" });
  }, [pick]);
  const row = (u: NavRef) => {
    const i = flat.indexOf(u);
    return (
      <li key={`${u.path}:${u.line}:${u.col}`} className={i === pick ? "picked" : ""} onMouseEnter={() => i >= 0 && setPick(i)} onClick={() => onGo(u)}>
        <span className="mono dim nav-use-at">
          {u.path}:{u.line}
          {changed(u.path, u.line) && <span className="nav-use-changed"> changed</span>}
        </span>
        <code className="nav-sig">{u.text}</code>
      </li>
    );
  };
  const count = uses.uses.filter((u) => !u.maybe).length;
  return (
    <aside className="nav-panel" aria-label={`Who uses ${uses.name}`}>
      <header className="nav-panel-head">
        <span>
          Who uses <code>{uses.name}</code>
        </span>
        <span className="dim">
          {count} {count === 1 ? "use" : "uses"}
          {uses.partial ? ", maybe more" : ""} · {sideWords(at.side)}
        </span>
        <span className="spacer-left" />
        <Button variant="ghost" size="small" onClick={onBack} title="Back (b)">
          <Icon name="arrow-left" size={12} /> <Keys k="b" />
        </Button>
        <Button variant="ghost" size="small" onClick={onClose} title="Close (esc)">
          <Icon name="x" size={13} />
        </Button>
      </header>
      <div className="nav-panel-body nav-uses">
        {uses.def && (
          <div className="nav-uses-def">
            <code className="nav-sig">{uses.def.signature || uses.name}</code>
            <span className="mono dim">
              {uses.def.path}:{uses.def.line}
            </span>
          </div>
        )}
        {!groups.length && <div className="dim nav-panel-loading">Nothing else uses it.</div>}
        {groups.map((g) => (
          <section key={g.title}>
            {(groups.length > 1 || g.title !== "Elsewhere") && <h3 className="nav-uses-title">{g.title}</h3>}
            <ul className="nav-list">{g.uses.map(row)}</ul>
            {g.maybes.length > 0 && (
              <>
                <Button variant="ghost" size="small" className="nav-maybes" onClick={() => setOpen((o) => ({ ...o, [g.title]: !o[g.title] }))} title="Uses that weren't followed to it, but could be it: one of several by that name, or an interface's method it may satisfy">
                  {open[g.title] ? "Hide" : "Show"} {g.maybes.length} that may be
                </Button>
                {open[g.title] && <ul className="nav-list maybes">{g.maybes.map(row)}</ul>}
              </>
            )}
          </section>
        ))}
      </div>
    </aside>
  );
}
