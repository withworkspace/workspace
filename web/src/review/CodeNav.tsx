import { useCallback, useEffect, useRef, useState } from "react";
import type { DiffFile } from "../api";
import { toast } from "../ui/toast";
import { CodeNavCard } from "./CodeNavCard";
import { CodeNavPicker } from "./CodeNavPicker";
import type { FileState, Row } from "./model";
import { caretAt, gapOf, getDefinition, getNames, getReferences, nameAt, rangeOf, rowOf, textOffset, type NavAnswer, type NavReferences, type NavTarget, type Place, type Side } from "./nav";
import { NavPanel } from "./NavPanel";
import { rowEl } from "./rowEl";
import { UsesPanel } from "./UsesPanel";

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
  return { rev, path, line: Number(line), side: side === "old" ? "old" : "new", inPanel: !!code.closest("[data-nav-panel]") };
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
        if (!refs.def) return toast(`${refs.name} not defined in the repo`);
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
        toast(`No definition of ${a.name} found`);
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
      el: rowEl(cursorRow.key),
    };
  };

  // d: the names on the cursor's line, to pick one of.
  const pickOnRow = async () => {
    const at = cursorLine();
    if (!at) {
      if (props.current.line === undefined && props.current.open) toast("Select a line first: click its number");
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
    if (!on.length) return toast("No names on this line");
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
        return toast("Nothing to go back to");
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
    <CodeNavCard
      answer={card.answer}
      rect={card.rect}
      pinned={card.pinned}
      definedHere={card.place.part === "d"}
      onClose={() => setCard(undefined)}
      onGo={(t) => (card.place.part === "d" ? usesOf(card.place) : go(t, card.place.side, card.answer?.how, card.place.inPanel))}
      onFollow={() => (card.place.part === "d" ? usesOf(card.place) : card.answer && follow(card.answer, card.place, card.rect))}
    />
  );
  const pickEl = picking && (
    <CodeNavPicker
      names={picking.names}
      rect={picking.rect}
      onClose={() => setPicking(undefined)}
      onPick={(i) => {
        const n = picking.names[i];
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
