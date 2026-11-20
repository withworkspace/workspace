import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import {
  act,
  approveDoc,
  type ArtifactContent,
  type DocQuote,
  draftWaiting,
  getThreads,
  getThreadVersion,
  openThread,
  pendingSuggestion,
  placeOf,
  recordDecision,
  type Suggestion,
  sayOnThread,
  sendThreads,
  setThreadStatus,
  settleSuggestion,
  type Thread,
  type ThreadKind,
  type ThreadState,
  threadKinds,
  waitingOnYou,
  withCrew,
} from "../api";
import { useKeys } from "../keys";
import { type TrackedChange, TrackedDoc } from "../editor/LazyTracked";
import { Markdown } from "../markdown/LazyMarkdown";
import { ReplyBox } from "../review/parts";
import { Icon } from "../ui/Icon";
import { Callout, humanError, Keys, toast, useAutosize, useMedia } from "../ui/kit";
import { exact, stamp } from "../ui/time";
import { BLOCK, blockLike, blockLines, blockSpan, blockWithLines, buildIndex, find, offsetOf, rangeOf, type Target, type TextIndex, targetOf, targetOfRange } from "./anchor";
import { changes } from "./diff";
import { useLatest, useLoaded } from "../load";
import { Chip } from "../ui/Chip";

// Commenting on a document, as in a shared doc: you read it rendered,
// highlight text and comment on it, and the comment is a thread, live at
// once. Threads sit in the margin level with what they're on. Crew answer
// in them (ws thread reply); you reply, and resolve them when you're done.
// What's changed since you last commented is marked in the left margin.
//
// Highlights are drawn with the CSS Custom Highlight API, so the rendered
// markdown (React's) is never touched: a thread is a Range over it.

export const annotateKeys: [string, string][] = [
  ["select, then c", "Comment on the text selected"],
  ["j / k", "Next / previous paragraph"],
  ["⇧↓ / ⇧↑", "Take in the next / previous paragraph"],
  ["c", "Comment on the paragraphs (or the text selected)"],
  ["n / p", "Next / previous open thread"],
  ["⏎", "Reply"],
  ["a / d", "Accept / decline the thread's suggested edit"],
  ["v", "See the suggested edits in place, as tracked changes"],
  ["x", "Resolve / reopen the thread"],
  ["i", "Show / hide what's changed since you last commented"],
  ["⇧A", "Approve the document"],
  ["tab", "While writing: ask, change or note"],
];

const kindWords: Record<ThreadKind, string> = { ask: "Ask", change: "Change", note: "Note" };
const kindIcon: Record<ThreadKind, "question" | "pencil" | "message"> = { ask: "question", change: "pencil", note: "message" };
const kindHints: Record<ThreadKind, string> = {
  ask: "What do you want to know?",
  change: "What should change?",
  note: "What do you want to say?",
};

// Spot is where a thread is drawn: a range of text, or a whole block with
// no words of its own (a diagram).
type Spot = { kind: "text"; start: number; end: number; range: Range } | { kind: "block"; el: HTMLElement };

// Pick is what a new thread is on.
interface Pick {
  line: number;
  endLine: number;
  quote: DocQuote;
  spot: Spot;
}

interface Placed {
  thread: Thread;
  spot?: Spot; // none: not found in this version
  pos: number; // where it is in the text, to order by
}

const COMPOSER = "composer";
const GAP = 8;

interface Box {
  top: number;
  height: number;
}

// Geo is where everything over the document goes, measured after it's
// drawn: the margin's cards, the cursor, the comment button, the marks on
// commented diagrams, and the bars by what's changed.
interface Geo {
  cards: Record<string, number>;
  height: number;
  cursor?: Box;
  button?: number;
  marks: (Box & { id: string; left: number; width: number })[];
  bars: (Box & { i: number; removed: boolean })[];
  // anchors are where each thread's text starts, for the markers that stand
  // in for the margin on a narrow window.
  anchors?: { id: string; top: number }[];
}

const noGeo: Geo = { cards: {}, height: 0, marks: [], bars: [] };

function spotRect(s: Spot): DOMRect | undefined {
  if (s.kind === "block") return s.el.isConnected ? s.el.getBoundingClientRect() : undefined;
  const rects = s.range.getClientRects();
  for (const r of rects) if (r.width > 0) return r;
  return rects[0];
}

function pickOf(ix: TextIndex, t: Target): Pick | undefined {
  const range = rangeOf(ix, t.start, t.end);
  return range && { line: t.line, endLine: t.endLine, quote: t.quote, spot: { kind: "text", start: t.start, end: t.end, range } };
}

// stack places cards level with what they're on, without overlapping:
// the one in focus exactly, the rest pushed away from it.
function stack(want: { id: string; top: number }[], height: (id: string) => number, focus?: string): Record<string, number> {
  const pos = want.map((w) => w.top);
  const f = Math.max(0, want.findIndex((w) => w.id === focus));
  for (let i = f + 1; i < want.length; i++) pos[i] = Math.max(want[i].top, pos[i - 1] + height(want[i - 1].id) + GAP);
  for (let i = f - 1; i >= 0; i--) pos[i] = Math.min(want[i].top, pos[i + 1] - height(want[i].id) - GAP);
  const lift = pos.length && pos[0] < 0 ? -pos[0] : 0;
  return Object.fromEntries(want.map((w, i) => [w.id, Math.round(pos[i] + lift)]));
}

const scroller = () => document.querySelector("main");

function caretAt(x: number, y: number): { node: Node; offset: number } | undefined {
  const p = document.caretPositionFromPoint?.(x, y);
  if (p) return { node: p.offsetNode, offset: p.offset };
  const r = (document as Document & { caretRangeFromPoint?: (x: number, y: number) => Range | null }).caretRangeFromPoint?.(x, y);
  return r ? { node: r.startContainer, offset: r.startOffset } : undefined;
}

const clip = (s: string, n: number) => (s.length > n ? s.slice(0, n).trimEnd() + "…" : s);
const linesWord = (n: { line: number; endLine: number }) => (n.endLine > n.line ? `lines ${n.line}–${n.endLine}` : `line ${n.line}`);
const num = (id: string) => Number(id.slice(1));

// sinceYou is the version of the document you last commented on (a thread
// you opened, or a reply), and when.
function sinceYou(threads: Thread[]): { version: string; at: string } | undefined {
  let best: { version: string; at: string } | undefined;
  for (const t of threads)
    for (const turn of t.turns)
      if (turn.by === "you" && turn.version && (!best || new Date(turn.at).getTime() > new Date(best.at).getTime())) best = { version: turn.version, at: turn.at };
  return best;
}

export function AnnotatedDoc({
  slug,
  artifact,
  updated,
  onBack,
  onChanged,
}: {
  slug: string;
  artifact: ArtifactContent;
  updated?: string;
  onBack: () => void;
  onChanged: () => void; // the document's been changed here: read it again
}) {
  const ref = artifact.ref;
  // A document outside a mission (no slug) has threads, and nothing a
  // mission adds: approving it, a stage, crew to send a change to,
  // decisions to record. Accepting a suggested edit saves it as an edit
  // of yours is saved: where it's landed by that (the default branch of a
  // repo with no pull requests), it's asked for twice.
  const own = slug === "";
  const lands = own && artifact.saves?.how === "land";
  const [landing, setLanding] = useState<string>(); // the thread whose edit a second a lands
  // Where it is, for a file in a repo: its links to other files and its
  // images are read from there. (One object while it's the same place: the
  // reader draws again when it changes.)
  const place = useMemo(() => artifact.in && placeOf(artifact.in), [artifact.in?.repo, artifact.in?.worktree, artifact.in?.path]);
  const [st, setST] = useState<ThreadState>();
  const [error, setError] = useState<string>();
  const article = useRef<HTMLDivElement>(null);
  const docCol = useRef<HTMLDivElement>(null);
  const [ix, setIx] = useState<TextIndex>();
  const [, setTick] = useState(0);
  const [active, setActive] = useState<string>();
  useEffect(() => {
    // Asked of one thread: moving off it is a no.
    setLanding((l) => (l && l !== active ? undefined : l));
  }, [active]);
  const [hover, setHover] = useState<string>();
  const [selection, setSelection] = useState<Pick>();
  const [cursor, setCursor] = useState<{ from: number; to: number }>();
  const [composing, setComposing] = useState<Pick>();
  // On a narrow window there's no margin: threads are markers beside their
  // text, and the one you tap (or write) opens in a sheet from the bottom.
  const narrow = useMedia("(max-width: 860px)");
  const [sheet, setSheet] = useState<string>();
  const [replying, setReplying] = useState<string>();
  const [showChanges, setShowChanges] = useState(true);
  const [openChange, setOpenChange] = useState<number>();
  const [geo, setGeo] = useState<Geo>(noGeo);
  const [said, setSaid] = useState<string>();
  const [settling, setSettling] = useState<string>(); // the thread whose edit is being accepted or declined
  const [approving, setApproving] = useState(false);
  // previewing: the suggested edits shown in place (tracked changes),
  // this thread's marked.
  const [previewing, setPreviewing] = useState<string>();
  const [offerDismissed, setOfferDismissed] = useState(false);
  const cards = useRef(new Map<string, HTMLDivElement>());

  const fail = (e: Error) => setError(e.message);
  // What a write did is said for a moment, then gone.
  useEffect(() => {
    if (!said) return;
    toast(said, { tone: "done" });
    setSaid(undefined);
  }, [said]);
  // The threads, and again whenever the mission changes: crew answering
  // shows as they do.
  const begin = useLatest();
  const load = useCallback(() => {
    const fresh = begin();
    return getThreads(slug, ref).then(
      (x) => {
        if (!fresh()) return;
        setST(x);
        setError(undefined);
      },
      (e: Error) => fresh() && setError(e.message),
    );
  }, [slug, ref, begin]);
  useEffect(() => {
    load();
  }, [load, updated, artifact]);

  // Index the document whenever what's drawn changes (the markdown loads,
  // code is highlighted, diagrams are drawn), and measure again when its
  // size does.
  useEffect(() => {
    const el = article.current;
    if (!el) return;
    let frame = 0;
    const rebuild = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => setIx(buildIndex(el)));
    };
    rebuild();
    const mo = new MutationObserver(rebuild);
    mo.observe(el, { childList: true, subtree: true, characterData: true });
    const remeasure = () => setTick((t) => t + 1);
    const ro = new ResizeObserver(remeasure);
    ro.observe(el);
    window.addEventListener("resize", remeasure);
    return () => {
      cancelAnimationFrame(frame);
      mo.disconnect();
      ro.disconnect();
      window.removeEventListener("resize", remeasure);
    };
  }, []);

  const blocks = useMemo(() => (ix && article.current ? [...article.current.querySelectorAll<HTMLElement>(BLOCK)] : []), [ix]);
  const textLines = useMemo(() => artifact.text.replace(/\r\n/g, "\n").split("\n"), [artifact.text]);
  const threads = useMemo(() => st?.threads ?? [], [st]);

  // Every thread on the document, found in it as it is now.
  const placed = useMemo((): Placed[] => {
    const root = article.current;
    if (!ix || !root) return [];
    return threads.map((thread): Placed => {
      const a = thread.anchor;
      const same = a.version === artifact.version;
      if (!a.quote?.exact) {
        const el = same ? blockWithLines(root, a.line, a.endLine) : blockLike(root, textLines, a.excerpt?.[0] ?? "", a.line);
        return el ? { thread, spot: { kind: "block", el }, pos: offsetOf(ix, el, 0) } : { thread, pos: Infinity };
      }
      const hit = find(ix, a.quote, same ? a : undefined);
      const range = hit && rangeOf(ix, hit[0], hit[1]);
      return hit && range ? { thread, spot: { kind: "text", start: hit[0], end: hit[1], range }, pos: hit[0] } : { thread, pos: Infinity };
    });
  }, [threads, ix, artifact.version, textLines]);
  const shown = useMemo(() => placed.filter((p) => p.spot).sort((a, b) => a.pos - b.pos), [placed]);
  // Threads whose text isn't in this version: listed above it, while open.
  const lost = useMemo(() => placed.filter((p) => !p.spot && p.thread.status === "open"), [placed]);
  // n and p go through what's open, and decisions drafted to record:
  // resolved threads stay folded, a click away.
  const nav = useMemo(() => [...shown.filter((p) => p.thread.status === "open" || draftWaiting(p.thread)), ...lost], [shown, lost]);
  const at = nav.findIndex((p) => p.thread.id === active);
  // The thread in focus: one n and p go through, or a resolved one clicked.
  const current = at >= 0 ? nav[at] : placed.find((p) => p.thread.id === active);
  const waiting = threads.filter(waitingOnYou);

  // The text selected in the document, to comment on.
  useEffect(() => {
    let frame = 0;
    const changed = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => {
        const sel = document.getSelection();
        const root = article.current;
        if (!sel || sel.isCollapsed || !sel.rangeCount || !root || !ix) return setSelection(undefined);
        const r = sel.getRangeAt(0);
        if (!root.contains(r.commonAncestorContainer)) return setSelection(undefined);
        const t = targetOfRange(ix, r);
        setSelection(t && pickOf(ix, t));
      });
    };
    document.addEventListener("selectionchange", changed);
    return () => {
      cancelAnimationFrame(frame);
      document.removeEventListener("selectionchange", changed);
    };
  }, [ix]);

  // The paragraphs the cursor is on, to comment on from the keyboard.
  const cursorPick = useMemo((): Pick | undefined => {
    if (!cursor || !ix) return undefined;
    const lo = Math.min(cursor.from, cursor.to);
    const hi = Math.max(cursor.from, cursor.to);
    const a = blocks[lo];
    const b = blocks[hi];
    if (!a || !b) return undefined;
    const line = blockLines(a)[0];
    const endLine = blockLines(b)[1];
    const t = targetOf(ix, blockSpan(ix, a)[0], blockSpan(ix, b)[1]);
    const p = t && pickOf(ix, t);
    if (p) return { ...p, line, endLine };
    if (lo === hi && line) return { line, endLine, quote: { exact: "" }, spot: { kind: "block", el: a } }; // a diagram
    return undefined;
  }, [cursor, ix, blocks]);

  // Highlights: each thread's text by its kind, the one in focus brighter,
  // and what you're writing on.
  useEffect(() => {
    if (!("highlights" in CSS)) return;
    const groups: Record<string, Range[]> = { "ws-note-active": [], "ws-note-pending": [], "ws-note-suggest": [] };
    for (const k of threadKinds) groups[`ws-note-${k}`] = [];
    for (const p of shown) {
      if (p.spot?.kind !== "text") continue;
      const focus = p.thread.id === active || p.thread.id === hover;
      if (p.thread.status === "resolved" && !focus) continue;
      groups[`ws-note-${p.thread.kind}`].push(p.spot.range);
      if (focus) groups["ws-note-active"].push(p.spot.range);
      if (p.thread.status === "open" && pendingSuggestion(p.thread)) groups["ws-note-suggest"].push(p.spot.range);
    }
    if (composing?.spot.kind === "text") groups["ws-note-pending"].push(composing.spot.range);
    for (const [name, ranges] of Object.entries(groups)) {
      const h = new Highlight(...ranges);
      h.priority = name === "ws-note-pending" ? 3 : name === "ws-note-suggest" ? 2 : name === "ws-note-active" ? 1 : 0;
      CSS.highlights.set(name, h);
    }
    return () => {
      for (const name of Object.keys(groups)) CSS.highlights.delete(name);
    };
  }, [shown, active, hover, composing]);

  // What's changed since you last commented, when the document has.
  const base = useMemo(() => sinceYou(threads), [threads]);
  const since = base && base.version !== artifact.version ? base : undefined;
  const sinceVersion = since?.version;
  const { value: before } = useLoaded(sinceVersion ? () => getThreadVersion(slug, ref, sinceVersion).then((text) => ({ version: sinceVersion, text })) : null, [slug, ref, sinceVersion]);
  const diff = useMemo(() => (before && before.version === sinceVersion ? changes(before.text, artifact.text) : []), [before, sinceVersion, artifact.text]);

  // Measure where everything goes, after every draw (it's cheap, and
  // changes only when something moved).
  useLayoutEffect(() => {
    const col = docCol.current;
    if (!col) return;
    const base = col.getBoundingClientRect();
    const want: { id: string; top: number }[] = [];
    const marks: Geo["marks"] = [];
    for (const p of shown) {
      const r = spotRect(p.spot!);
      if (!r) continue;
      want.push({ id: p.thread.id, top: r.top - base.top });
      if (p.spot!.kind === "block") marks.push({ id: p.thread.id, top: r.top - base.top, height: r.height, left: r.left - base.left, width: r.width });
    }
    const pickRect = composing && spotRect(composing.spot);
    if (pickRect) want.push({ id: COMPOSER, top: pickRect.top - base.top });
    if (composing?.spot.kind === "block" && pickRect)
      marks.push({ id: COMPOSER, top: pickRect.top - base.top, height: pickRect.height, left: pickRect.left - base.left, width: pickRect.width });
    want.sort((a, b) => a.top - b.top);
    const focus = composing ? COMPOSER : active;
    const placedCards = stack(want, (id) => cards.current.get(id)?.offsetHeight ?? 72, focus);
    let height = col.offsetHeight;
    for (const w of want) height = Math.max(height, (placedCards[w.id] ?? 0) + (cards.current.get(w.id)?.offsetHeight ?? 72) + GAP);

    let cursorBox: Box | undefined;
    if (cursor && !composing) {
      const a = blocks[Math.min(cursor.from, cursor.to)]?.getBoundingClientRect();
      const b = blocks[Math.max(cursor.from, cursor.to)]?.getBoundingClientRect();
      if (a && b) cursorBox = { top: a.top - base.top, height: b.bottom - a.top };
    }
    let button: number | undefined;
    const sel = selection && spotRect(selection.spot);
    if (sel && !composing) button = sel.top - base.top;
    else if (cursorBox && !selection) button = cursorBox.top;

    const bars: Geo["bars"] = [];
    if (showChanges && diff.length && blocks.length) {
      diff.forEach((c, i) => {
        const removed = c.newEnd < c.newStart;
        const hits = removed ? [] : blocks.map((b) => Number(b.dataset.ls) <= c.newEnd && Number(b.dataset.le) >= c.newStart);
        if (hits.some(Boolean)) {
          // A bar for each run of changed blocks that sit together: a
          // change whose lines render apart (a footnote's definition, at
          // the end) doesn't mark what's between.
          const rects = blocks.map((b) => b.getBoundingClientRect());
          const order = blocks.map((_, j) => j).sort((x, y) => rects[x].top - rects[y].top);
          let run: number[] = [];
          const flush = () => {
            if (!run.length) return;
            const top = Math.min(...run.map((j) => rects[j].top)) - base.top;
            const bottom = Math.max(...run.map((j) => rects[j].bottom)) - base.top;
            bars.push({ i, top, height: bottom - top, removed: false });
            run = [];
          };
          for (const j of order) {
            if (hits[j]) run.push(j);
            else if (run.length && !blocks[j].contains(blocks[run[run.length - 1]]) && !blocks[run[run.length - 1]].contains(blocks[j])) flush();
          }
          flush();
          return;
        }
        // Lines only removed, or changed between blocks: a mark where the
        // next block starts, or at the end.
        const next = blocks.find((b) => Number(b.dataset.ls) >= c.newStart);
        const top = next ? next.getBoundingClientRect().top - base.top - 6 : col.offsetHeight;
        bars.push({ i, top, height: 4, removed: true });
      });
    }
    const anchors = want.filter((w) => w.id !== COMPOSER);
    const next: Geo = { cards: placedCards, height, cursor: cursorBox, button, marks, bars, anchors };
    setGeo((g) => (JSON.stringify(g) === JSON.stringify(next) ? g : next));
  });

  const stale = !!st && st.version !== artifact.version;

  const reveal = (p: Placed) => {
    const main = scroller();
    const r = p.spot ? spotRect(p.spot) : document.getElementById(`thread-${p.thread.id}`)?.getBoundingClientRect();
    if (!main || !r) return;
    const m = main.getBoundingClientRect();
    if (r.top < m.top + 60 || r.bottom > m.bottom - 60) main.scrollBy({ top: r.top - m.top - m.height / 3 });
  };
  const go = (i: number) => {
    const p = nav[Math.min(Math.max(i, 0), nav.length - 1)];
    if (!p) return;
    setActive(p.thread.id);
    setReplying(undefined);
    if (p.spot) {
      const b = p.spot.kind === "block" ? p.spot.el : (p.spot.range.startContainer.parentElement?.closest(BLOCK) as HTMLElement | null);
      const bi = b ? blocks.indexOf(b) : -1;
      setCursor(bi >= 0 ? { from: bi, to: bi } : undefined);
    }
    reveal(p);
  };
  // The first thread below the top of the page, for n with none in focus.
  const firstInView = () => {
    const top = scroller()?.getBoundingClientRect().top ?? 0;
    const i = nav.findIndex((p) => !p.spot || (spotRect(p.spot)?.top ?? 0) >= top);
    return i >= 0 ? i : 0;
  };
  const moveCursor = (step: number, extend = false) => {
    if (!blocks.length) return;
    let to: number;
    if (!cursor) {
      const top = scroller()?.getBoundingClientRect().top ?? 0;
      to = Math.max(0, blocks.findIndex((b) => b.getBoundingClientRect().bottom > top + 8));
    } else to = Math.min(Math.max(cursor.to + step, 0), blocks.length - 1);
    setCursor({ from: extend && cursor ? cursor.from : to, to });
    setActive(undefined);
    const r = blocks[to].getBoundingClientRect();
    const main = scroller();
    const m = main?.getBoundingClientRect();
    if (main && m && (r.top < m.top + 40 || r.bottom > m.bottom - 40)) main.scrollBy({ top: step > 0 ? r.bottom - m.bottom + 80 : r.top - m.top - 80 });
  };

  const comment = () => {
    const pick = selection ?? cursorPick;
    if (!pick || stale) return;
    setComposing(pick);
    setActive(undefined);
    setReplying(undefined);
    document.getSelection()?.removeAllRanges();
  };
  const post = async (kind: ThreadKind, body: string, suggestion?: string) => {
    if (!composing) return;
    const next = await openThread(slug, ref, {
      version: artifact.version,
      line: composing.line,
      endLine: composing.endLine,
      quote: composing.quote,
      kind,
      body,
      suggestion,
    });
    setST(next);
    const newest = [...next.threads].sort((a, b) => num(b.id) - num(a.id))[0];
    setActive(newest?.id);
    setComposing(undefined);
    setCursor(undefined);
  };
  const say = (id: string, body: string) =>
    sayOnThread(slug, ref, id, body).then((next) => {
      setST(next);
      setReplying(undefined);
    }, fail);
  const toggleResolved = (t?: Thread) =>
    t && setThreadStatus(slug, ref, t.id, t.status === "resolved" ? "open" : "resolved").then(setST, fail);
  // Accepting writes the edit in (and commits it, in a capsule): the
  // document's read again to show it. Declining asks you to say why.
  // In the tracked view, accepting goes on to the next edit; declining
  // leaves it, to say why.
  const settle = (t: Thread | undefined, action: "accept" | "decline") => {
    if (!t || !pendingSuggestion(t) || settling) return;
    if (lands && action === "accept" && landing !== t.id) {
      setLanding(t.id);
      setActive(t.id);
      return;
    }
    setLanding(undefined);
    setSettling(t.id);
    settleSuggestion(slug, ref, t.id, action)
      .finally(() => setSettling(undefined))
      .then((next) => {
        setST(next);
        setError(undefined);
        if (action === "accept") {
          setSaid(next.message);
          justAccepted.current = pendingSuggestion(t)?.to;
          setPreviewing((p) => (p ? next.threads.find((x) => x.status === "open" && pendingSuggestion(x))?.id : p));
          onChanged();
        } else {
          setPreviewing(undefined);
          setActive(t.id);
          setReplying(t.id);
        }
      }, fail);
  };

  // Approving says you're happy with the document as it is: refused while
  // a change you asked for is open. When it's what the current stage made,
  // accepting the stage is offered next.
  const approve = () => {
    if (approving || stale || own) return;
    const changes = threads.filter((t) => t.status === "open" && t.kind === "change").map((t) => t.id);
    if (changes.length) {
      const one = changes.length === 1;
      setError(`${changes.join(", ")} ${one ? "asks" : "ask"} for a change: resolve ${one ? "it" : "them"} first (declining a suggestion, if you don't want it).`);
      return;
    }
    setApproving(true);
    approveDoc(slug, ref, artifact.version)
      .finally(() => setApproving(false))
      .then((next) => {
        setST(next);
        setError(undefined);
        setOfferDismissed(false);
        setSaid(next.stage?.state === "current" ? undefined : next.message);
      }, fail);
  };
  // A change bigger than an edit goes to crew: the crew that wrote the
  // document, or fresh crew in its capsule. ids empty: every open change
  // crew don't have.
  const [sending, setSending] = useState(false);
  const sendToCrew = (ids: string[]) => {
    if (sending) return;
    setSending(true);
    sendThreads(slug, ref, ids)
      .finally(() => setSending(false))
      .then((next) => {
        setST(next);
        setError(undefined);
        setSaid(next.message);
      }, fail);
  };
  // What was decided in a resolved thread, recorded as a decision (about
  // the document and its stage), or its draft skipped.
  const decide = (t: Thread, d: { action: "record"; what: string; why: string } | { action: "skip" }) =>
    recordDecision(slug, ref, t.id, d).then((next) => {
      setST(next);
      setError(undefined);
      setSaid(next.message);
    }, fail);
  const acceptStage = () =>
    act(slug, "accept", { text: `approved ${artifact.ref.slice(artifact.ref.indexOf(":") + 1)}` }).then((r) => {
      setSaid(r.message);
      setError(undefined);
      load();
    }, fail);

  // A click on a thread's text brings it into focus; on a paragraph, puts
  // the cursor there.
  // An edit just accepted: once the document's reloaded with it, the
  // passage it changed glows for a moment, so you see it land.
  const justAccepted = useRef<string | undefined>(undefined);
  useEffect(() => {
    const to = justAccepted.current;
    const el = article.current;
    if (to === undefined || !el) return;
    justAccepted.current = undefined;
    const words = to.replace(/[`*_#>[\]()]/g, "").replace(/\s+/g, " ").trim().slice(0, 48);
    const t = setTimeout(() => {
      const hit = [...el.querySelectorAll<HTMLElement>("[data-block]")].filter((b) => words && b.textContent?.replace(/\s+/g, " ").includes(words));
      for (const b of hit) {
        b.classList.add("just-changed");
        setTimeout(() => b.classList.remove("just-changed"), 2400);
      }
      hit[0]?.scrollIntoView({ block: "nearest", behavior: "smooth" });
    }, 120);
    return () => clearTimeout(t);
  }, [artifact.text]);

  // Hovering a thread's text lights its card in the margin, as hovering
  // the card lights its text.
  const hoverFromText = useRef(false);
  const hoverFrame = useRef(0);
  const textHover = (e: React.MouseEvent) => {
    const { clientX: x, clientY: y } = e;
    cancelAnimationFrame(hoverFrame.current);
    hoverFrame.current = requestAnimationFrame(() => {
      const at = caretAt(x, y);
      let id: string | undefined;
      if (at) {
        for (const p of shown) {
          if (p.spot?.kind !== "text") continue;
          try {
            if (p.spot.range.isPointInRange(at.node, at.offset)) {
              id = p.thread.id;
              break;
            }
          } catch {
            // a point in another document: not in it
          }
        }
      }
      if (id || hoverFromText.current) {
        hoverFromText.current = !!id;
        setHover((h) => (h === id ? h : id));
      }
    });
  };
  const docClick = (e: React.MouseEvent) => {
    if (!document.getSelection()?.isCollapsed) return; // the end of a drag, selecting
    if ((e.target as Element).closest("a")) return;
    const caret = caretAt(e.clientX, e.clientY);
    let hit: Placed | undefined;
    let size = Infinity;
    for (const p of shown) {
      const s = p.spot!;
      const inside = s.kind === "block" ? s.el.contains(e.target as Node) : !!caret && s.range.isPointInRange(caret.node, caret.offset);
      const n = s.kind === "text" ? s.end - s.start : 1e9;
      if (inside && (p.thread.status === "open" || p.thread.id === active) && n < size) {
        hit = p;
        size = n;
      }
    }
    setActive(hit?.thread.id);
    setReplying(undefined);
    const b = (e.target as Element).closest(BLOCK) as HTMLElement | null;
    const bi = b ? blocks.indexOf(b) : -1;
    setCursor(bi >= 0 ? { from: bi, to: bi } : undefined);
  };

  const escape = () => {
    if (previewing) setPreviewing(undefined);
    else if (openChange !== undefined) setOpenChange(undefined);
    else if (selection) document.getSelection()?.removeAllRanges();
    else if (replying) setReplying(undefined);
    else if (active) setActive(undefined);
    else if (cursor) setCursor(undefined);
    else onBack();
  };
  useKeys({
    j: () => moveCursor(1),
    ArrowDown: () => moveCursor(1),
    k: () => moveCursor(-1),
    ArrowUp: () => moveCursor(-1),
    "shift+ArrowDown": () => moveCursor(1, true),
    "shift+ArrowUp": () => moveCursor(-1, true),
    c: comment,
    n: () => go(at < 0 ? firstInView() : at + 1),
    p: () => go(at < 0 ? firstInView() - 1 : at - 1),
    Enter: () => current && setReplying(current.thread.id),
    x: () => toggleResolved(current?.thread),
    a: () => settle(preview?.thread ?? current?.thread, "accept"),
    d: () => settle(preview?.thread ?? current?.thread, "decline"),
    i: () => since && setShowChanges((v) => !v),
    A: approve,
    v: () => {
      if (previewing) setPreviewing(undefined);
      else if (current && pendingSuggestion(current.thread)) setPreviewing(current.thread.id);
      else if (edits.length) setPreviewing(edits[0].id);
    },
    // Esc clears what's picked out first; with nothing, goes back.
    Escape: escape,
    h: onBack,
  });

  const open = threads.filter((t) => t.status === "open");
  const answering = threads.filter((t) => t.answering && t.status === "open");
  const drafts = threads.filter(draftWaiting);
  // The suggested edits waiting, to show in place; the one marked.
  const edits = useMemo<TrackedChange[]>(
    () =>
      threads.flatMap((t) => {
        const sg = t.status === "open" ? pendingSuggestion(t) : undefined;
        return sg ? [{ id: t.id, from: sg.from, to: sg.to, by: sg.by }] : [];
      }),
    [threads],
  );
  const preview = useMemo(() => {
    const t = previewing ? threads.find((x) => x.id === previewing) : undefined;
    return t && pendingSuggestion(t) ? { thread: t } : undefined;
  }, [previewing, threads]);
  const toSend = open.filter((t) => t.kind === "change" && !withCrew(t) && !t.answering && !pendingSuggestion(t));
  const approved = st?.approved;
  const approvedNow = !!approved && approved.version === artifact.version;
  const stage = st?.stage;
  const offerStage = approvedNow && stage?.state === "current" && !offerDismissed;
  const setCard = (id: string) => (el: HTMLDivElement | null) => {
    if (el) cards.current.set(id, el);
    else cards.current.delete(id);
  };
  const card = (p: Placed, unplaced = false) => (
    <ThreadCard
      thread={p.thread}
      active={p.thread.id === active}
      hovered={p.thread.id === hover && p.thread.id !== active}
      replying={replying === p.thread.id}
      unplaced={unplaced}
      onFocus={() => setActive(p.thread.id)}
      onHover={(on) => setHover(on ? p.thread.id : undefined)}
      onReply={() => {
        setActive(p.thread.id);
        setReplying(p.thread.id);
      }}
      onSay={(body) => say(p.thread.id, body)}
      onReplyCancel={() => setReplying(undefined)}
      onToggle={() => toggleResolved(p.thread)}
      onSettle={(action) => settle(p.thread, action)}
      settling={settling === p.thread.id}
      onSend={() => sendToCrew([p.thread.id])}
      sending={sending}
      own={own}
      landing={landing === p.thread.id ? artifact.saves?.branch : undefined}
      onDecide={(d) => decide(p.thread, d)}
      onPreview={() => setPreviewing(p.thread.id)}
    />
  );

  return (
    <div className="annotating-doc">
      <div className="annotate-bar">
        <span className="annotate-counts">
          {open.length > 0 && (
            <Chip tone="quiet">
              <Icon name="message" size={11} /> {open.length} open
            </Chip>
          )}
          {waiting.length > 0 && (
            <Chip tone="attention">
              <Icon name="reply" size={11} /> {waiting.length} your turn
            </Chip>
          )}
          {answering.length > 0 && (
            <Chip tone="progress">
              <Icon name="spinner" size={11} className="spin" /> {answering.length} answering
            </Chip>
          )}
          {drafts.length > 0 && (
            <Chip tone="attention">
              <Icon name="gavel" size={11} /> {drafts.length} {drafts.length === 1 ? "decision" : "decisions"} to record
            </Chip>
          )}
        </span>
        <span className="spacer-left" />
        {!own && toSend.length > 1 && (
          <button
            disabled={sending || stale}
            onClick={() => sendToCrew(toSend.map((t) => t.id))}
            title={`Hand ${toSend.map((t) => t.id).join(", ")} to crew: the crew that wrote it, or fresh crew in its capsule`}
          >
            Send {toSend.length} changes to crew
          </button>
        )}
        {since && diff.length > 0 && (
          <button className="ghost small" onClick={() => setShowChanges((v) => !v)} aria-pressed={showChanges} title="Show or hide what's changed since you commented (i)">
            <Icon name="eye" size={13} /> {showChanges ? "Hide changes" : "Show changes"} <Keys k="i" />
          </button>
        )}
        {own ? null : approvedNow ? (
          <Chip tone="done" className="approved" title={`You approved this version ${exact(approved.at)}`}>
            <Icon name="check-circle" size={12} /> Approved
          </Chip>
        ) : (
          <>
            {approved && <span className="dim approved-before">You approved an earlier version</span>}
            <button className={open.some((t) => t.kind === "change") ? "" : "primary"} disabled={approving || stale} onClick={approve} title="Approve the document as it is (⇧A)">
              <Icon name="check" size={13} /> {approved ? "Approve again" : "Approve"} <Keys k="⇧A" />
            </button>
          </>
        )}
      </div>
      {offerStage && stage && (
        <Callout
          tone="done"
          title="Approved"
          actions={
            <>
              <button className="primary" onClick={acceptStage}>
                Accept stage {stage.n}
              </button>
              <button className="ghost" onClick={() => setOfferDismissed(true)}>
                Not now
              </button>
            </>
          }
        >
          It's what stage {stage.n} ({stage.title}) set out to make: accept the stage too?
        </Callout>
      )}
      {stale && (
        <Callout
          tone="attention"
          title="This document has changed since it loaded"
          actions={
            <button onClick={onChanged}>
              Reload <Keys k="r" />
            </button>
          }
        >
          New threads wait until you've seen the latest.
        </Callout>
      )}
      {since && diff.length > 0 && showChanges && (
        <Callout tone="info" icon="eye" title={`${diff.length} ${diff.length === 1 ? "change" : "changes"} since you commented ${stamp(since.at).replace(/^Yesterday/, "yesterday")}`}>
          Marked in the left margin: click one to see what it said before.
          {waiting.length > 0 && (
            <>
              {" "}
              {waiting.length} {waiting.length === 1 ? "thread is" : "threads are"} waiting on you: <Keys k="n" /> goes through them.
            </>
          )}
        </Callout>
      )}
      {error && <div className="banner">{humanError(error)}</div>}
      {lost.length > 0 && (
        <section className="lost-notes">
          <div className="dim">
            {lost.length === 1 ? "A thread's" : `${lost.length} threads'`} text isn't in this version: it's been changed, or removed.
          </div>
          {lost.map((p) => (
            <div key={p.thread.id} id={`thread-${p.thread.id}`}>
              {card(p, true)}
            </div>
          ))}
        </section>
      )}
      {preview && (
        <Callout tone="info" icon="eye" title={edits.length === 1 ? "The suggested edit, in place" : `${edits.length} suggested edits, in place`} className="preview-bar">
          Struck through is what an edit takes out, underlined what it puts in. <Keys k="a" /> accepts the one marked ({preview.thread.id}), <Keys k="d" /> declines it; click another to mark it.{" "}
          <Keys k="v" /> or <Keys k="esc" /> goes back.
        </Callout>
      )}
      {preview && (
        <div className="annotate preview">
          <div className="doc-col">
            <article className="artifact-body">
              <TrackedDoc
                text={artifact.text}
                changes={edits}
                focus={preview.thread.id}
                settling={settling}
                onFocus={setPreviewing}
                onSettle={(id, action) => settle(threads.find((x) => x.id === id), action)}
              />
            </article>
          </div>
        </div>
      )}
      <div className="annotate" style={preview ? { display: "none" } : undefined}>
        <div className="doc-col" ref={docCol}>
          <article className="artifact-body" ref={article} onClick={docClick} onMouseMove={textHover} onMouseLeave={() => hoverFromText.current && setHover(undefined)}>
            <Markdown text={artifact.text} lines at={place} />
          </article>
          {geo.cursor && <div className="doc-cursor" style={{ top: geo.cursor.top, height: geo.cursor.height }} />}
          {geo.marks.map((m) => (
            <div
              key={m.id}
              className={`block-mark${m.id === active || m.id === COMPOSER || m.id === hover ? " on" : ""}`}
              style={{ top: m.top - 4, height: m.height + 8, left: m.left - 4, width: m.width + 8 }}
            />
          ))}
          {narrow &&
            (geo.anchors ?? []).map((a, i, all) => {
              const p = shown.find((x) => x.thread.id === a.id);
              if (!p) return null;
              const t = p.thread;
              const stacked = all.slice(0, i).filter((b) => Math.abs(b.top - a.top) < 20).length;
              return (
                <button
                  key={a.id}
                  className={`thread-marker k-${t.kind}${t.status === "resolved" ? " resolved" : ""}${waitingOnYou(t) ? " yours" : ""}${a.id === active ? " on" : ""}`}
                  style={{ top: a.top + stacked * 24 }}
                  onClick={(e) => {
                    e.stopPropagation();
                    setActive(a.id);
                    setSheet(a.id);
                  }}
                  aria-label={`${kindWords[t.kind]} thread ${t.id}${t.status === "resolved" ? ", resolved" : ""}`}
                >
                  <Icon name={t.status === "resolved" ? "check" : kindIcon[t.kind]} size={11} />
                  {t.turns.length > 1 && <span>{t.turns.length}</span>}
                </button>
              );
            })}
          {geo.bars.map((b) => (
            <button
              key={b.i}
              className={`change-bar${b.removed ? " removed" : ""}${openChange === b.i ? " on" : ""}`}
              style={{ top: b.top, height: b.height }}
              title={b.removed ? "Removed since you last commented" : "Changed since you last commented"}
              onClick={() => setOpenChange((o) => (o === b.i ? undefined : b.i))}
            />
          ))}
          {openChange !== undefined && diff[openChange] && (
            <ChangePop
              top={(geo.bars.find((b) => b.i === openChange)?.top ?? 0) + (geo.bars.find((b) => b.i === openChange)?.height ?? 0) + 6}
              old={diff[openChange].old}
              added={diff[openChange].newEnd >= diff[openChange].newStart}
              onClose={() => setOpenChange(undefined)}
            />
          )}
        </div>
        <div className="margin" style={{ height: geo.height }}>
          {geo.button !== undefined && !composing && !stale && (
            <button
              className={`add-comment${selection ? " text" : ""}`}
              style={{ top: geo.button }}
              onMouseDown={(e) => e.preventDefault()}
              onClick={comment}
              title="Comment (c)"
            >
              {selection ? (
                <>
                  <Icon name="message" size={13} /> Comment <Keys k="c" />
                </>
              ) : (
                <>
                  <Icon name="plus" size={12} /> <Keys k="c" />
                </>
              )}
            </button>
          )}
          {shown.map((p) => (
            <div
              key={p.thread.id}
              ref={setCard(p.thread.id)}
              className="slot"
              style={{ top: geo.cards[p.thread.id] ?? 0, opacity: geo.cards[p.thread.id] === undefined ? 0 : undefined }}
            >
              {card(p)}
            </div>
          ))}
          {!shown.length && !composing && geo.button === undefined && !stale && (
            <div className="margin-hint">
              <Icon name="message" size={14} />
              <span>
                Select text to comment on it, or move with <Keys k="j" /> <Keys k="k" /> and press <Keys k="c" />. Ask a question, ask for a change, or leave a note:
                an agent answers in the thread.
              </span>
            </div>
          )}
          {composing && !narrow && (
            <div key={COMPOSER} ref={setCard(COMPOSER)} className="slot" style={{ top: geo.cards[COMPOSER] ?? 0, opacity: geo.cards[COMPOSER] === undefined ? 0 : undefined }}>
              <Composer pick={composing} onPost={post} onCancel={() => setComposing(undefined)} />
            </div>
          )}
        </div>
      </div>
      {narrow && (composing || (sheet && shown.some((p) => p.thread.id === sheet))) && (
        <>
          <div
            className="sheet-scrim"
            onClick={() => {
              setSheet(undefined);
              setComposing(undefined);
            }}
          />
          <div className="sheet" role="dialog" aria-modal="true" aria-label={composing ? "New comment" : "Thread"}>
            <div className="sheet-head">
              <span className="sheet-grip" aria-hidden />
              <button
                className="ghost icon-button small"
                onClick={() => {
                  setSheet(undefined);
                  setComposing(undefined);
                }}
                aria-label="Close"
              >
                <Icon name="x" size={14} />
              </button>
            </div>
            {composing ? (
              <Composer pick={composing} onPost={post} onCancel={() => setComposing(undefined)} />
            ) : (
              card(shown.find((p) => p.thread.id === sheet)!)
            )}
          </div>
        </>
      )}
    </div>
  );
}

interface CardProps {
  thread: Thread;
  active: boolean;
  hovered?: boolean;
  replying: boolean;
  unplaced: boolean;
  onFocus: () => void;
  onHover: (on: boolean) => void;
  onReply: () => void;
  onSay: (body: string) => void;
  onReplyCancel: () => void;
  onToggle: () => void;
  onSettle: (action: "accept" | "decline") => void;
  settling: boolean;
  onSend: () => void;
  own: boolean; // the document's in no mission: no crew to send to, no decisions
  landing?: string; // accepting its edit lands it on this branch: asked once, a second accept does it
  sending: boolean;
  onPreview: () => void;
  onDecide: (d: { action: "record"; what: string; why: string } | { action: "skip" }) => Promise<void>;
}

// ThreadCard is a thread in the margin: what you said, and what's been
// said since. A resolved one is folded until it's in focus.
function ThreadCard({ thread, active, hovered, replying, unplaced, onFocus, onHover, onReply, onSay, onReplyCancel, onToggle, onSettle, settling, onSend, sending, own, landing, onDecide, onPreview }: CardProps) {
  const resolved = thread.status === "resolved";
  const pending = !resolved ? pendingSuggestion(thread) : undefined;
  const settled = (thread.suggestions ?? []).filter((s) => s.state === "accepted" || s.state === "declined");
  const drafted = draftWaiting(thread);
  const [deciding, setDeciding] = useState(false); // writing a decision of your own
  const folded = resolved && !active && !unplaced && !drafted;
  const yours = waitingOnYou(thread);
  const crew = !resolved && withCrew(thread) ? thread.sent?.crew : undefined;
  const sendable = !own && !resolved && thread.kind === "change" && !crew && !thread.answering && !pending;
  const [first, ...rest] = thread.turns;
  return (
    <div
      className={`note-card doc-note k-${thread.kind}${active ? " cursor" : ""}${hovered ? " hovered" : ""}${resolved ? " resolved" : ""}${folded ? " folded" : ""}${yours ? " yours" : ""}`}
      onClick={onFocus}
      onMouseEnter={() => onHover(true)}
      onMouseLeave={() => onHover(false)}
    >
      <div className="note-head">
        <Chip className={`kind-chip k-${thread.kind}`}>
          <Icon name={kindIcon[thread.kind]} size={11} />
          {kindWords[thread.kind]}
        </Chip>
        {resolved ? (
          <Chip tone="done">
            <Icon name="check" size={11} /> Resolved
          </Chip>
        ) : crew ? (
          <Chip tone="progress">{crew} is on it</Chip>
        ) : yours ? (
          <Chip tone="attention">Your turn</Chip>
        ) : null}
        <span className="spacer-left" />
        <span className="note-id">{thread.id}</span>
        <button className="ghost small note-toggle" onClick={(e) => (e.stopPropagation(), onFocus(), onToggle())} title={`${resolved ? "Reopen" : "Resolve"} (x)`}>
          {resolved ? "Reopen" : "Resolve"}
        </button>
      </div>
      {folded ? (
        <div className="note-body dim one-line">{first?.body}</div>
      ) : (
        <>
          {unplaced && (
            <blockquote className="lost-quote" title={`opened on version ${thread.anchor.version} of the document, ${linesWord(thread.anchor)}`}>
              {thread.anchor.quote?.exact ? clip(thread.anchor.quote.exact, 240) : `The whole of ${linesWord(thread.anchor)}`}
            </blockquote>
          )}
          {first && <div className="note-body">{first.body}</div>}
          {rest.length > 0 && (
            <ul className="thread">
              {rest.map((t) => (
                <li key={t.n} className={`by-${t.by === "you" || t.by === "agent" || t.by === "ws" ? t.by : "crew"}`}>
                  <span className="who">{t.by === "you" ? "You" : t.by === "agent" ? "Agent" : t.by}</span>
                  {t.changed && <span className="dim">changed it</span>}
                  {t.commit && <span className="mono dim">{t.commit.slice(0, 7)}</span>}
                  <span className="said">{t.by === "you" ? t.body : <Markdown text={t.body} pinned={t.refs} />}</span>
                </li>
              ))}
            </ul>
          )}
          {thread.answering && (
            <div className="typing">
              <span className="typing-dots" aria-hidden>
                <span />
                <span />
                <span />
              </span>
              {resolved ? "Drafting what was decided" : "Answering"}
            </div>
          )}
          {thread.sent?.finished && !resolved && (
            <div className="dim settled">{thread.sent.crew} finished without saying it changed the document for this.</div>
          )}
          {settled.map((s) => (
            <div key={s.n} className="dim settled">
              {s.state === "accepted" ? `Edit accepted${s.commit ? `, committed ${s.commit.slice(0, 7)}` : ""}` : "Edit declined"}
            </div>
          ))}
          {pending && <EditCard s={pending} active={active} onSettle={onSettle} settling={settling} onPreview={onPreview} landing={landing} />}
          {drafted && thread.draft && <DecisionCard key={thread.draft.at} draft={thread.draft} onDecide={onDecide} />}
          {thread.draft?.state === "recorded" && thread.draft.decision && <div className="dim settled">Recorded as {thread.draft.decision}</div>}
          {!own && resolved && !thread.draft && !thread.answering && active && !deciding && (
            <div className="dim reply-hint">
              <button className="link" onClick={(e) => (e.stopPropagation(), setDeciding(true))}>
                Record what was decided
              </button>
            </div>
          )}
          {deciding && !thread.draft && <DecisionCard onDecide={(d) => onDecide(d).then(() => setDeciding(false))} onCancel={() => setDeciding(false)} />}
          {!replying && active && (
            <div className="dim reply-hint">
              <button className="link" onClick={(e) => (e.stopPropagation(), onReply())}>
                <Icon name="reply" size={12} /> Reply <Keys k="⏎" />
              </button>
              {sendable && (
                <button
                  className="link"
                  disabled={sending}
                  title="Too big for an edit: hand it to crew (the crew that wrote it, or fresh crew in its capsule)"
                  onClick={(e) => (e.stopPropagation(), onSend())}
                >
                  Send to crew
                </button>
              )}
            </div>
          )}
          {replying && <ReplyBox onSend={onSay} onCancel={onReplyCancel} />}
        </>
      )}
    </div>
  );
}

// EditCard is a suggested edit: what the markdown says, what it would
// say, and accepting (written in, and committed in a capsule) or declining.
function EditCard({
  s,
  active,
  onSettle,
  settling,
  onPreview,
  landing,
}: {
  s: Suggestion;
  active: boolean;
  onSettle: (action: "accept" | "decline") => void;
  settling: boolean;
  onPreview: () => void;
  landing?: string;
}) {
  return (
    <div className="doc-suggestion edit" onClick={(e) => e.stopPropagation()}>
      <div className="suggestion-head">
        <Icon name="pencil" size={12} /> Suggested edit <span className="dim">by {s.by === "agent" ? "the agent" : s.by}</span>
      </div>
      <div className="sl d">{s.from}</div>
      {s.to === "" ? <div className="sl dim">(delete it)</div> : <div className="sl a">{s.to}</div>}
      {landing && (
        <div className="dim settled">
          Accepting writes it in and lands it on {landing} at once: there's no pull request. <Keys k="a" /> again does it.
        </div>
      )}
      <div className="note-actions">
        <button className="link see-applied" onClick={onPreview} title="See it in the document, as a tracked change (v)">
          <Icon name="eye" size={12} /> See it in place
        </button>
        <span className="spacer-left" />
        {settling ? (
          <span className="dim busy">
            <Icon name="spinner" className="spin" /> Writing it in…
          </span>
        ) : (
          <>
            <button className="ghost small" disabled={settling} onClick={() => onSettle("decline")} title="Decline it (d)">
              Decline {active && <Keys k="d" />}
            </button>
            <button className="primary small" disabled={settling} onClick={() => onSettle("accept")} title={landing ? `Accept it, and land it on ${landing} (a)` : "Accept it (a)"}>
              {landing ? `Land it on ${landing}` : "Accept"} {active && <Keys k="a" />}
            </button>
          </>
        )}
      </div>
    </div>
  );
}

// DecisionCard records what was decided in a resolved thread: as its agent
// drafted it (skip it, or edit and record it), or in your own words.
function DecisionCard({
  draft,
  onDecide,
  onCancel,
}: {
  draft?: NonNullable<Thread["draft"]>;
  onDecide: (d: { action: "record"; what: string; why: string } | { action: "skip" }) => Promise<void>;
  onCancel?: () => void;
}) {
  const [what, setWhat] = useState(draft?.what ?? "");
  const [why, setWhy] = useState(draft?.why ?? "");
  const [busy, setBusy] = useState(false);
  const run = (d: { action: "record"; what: string; why: string } | { action: "skip" }) => {
    if (busy || (d.action === "record" && !d.what.trim())) return;
    setBusy(true);
    onDecide(d).finally(() => setBusy(false));
  };
  const keys = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
      e.preventDefault();
      run({ action: "record", what, why });
    }
  };
  return (
    <div className="doc-suggestion decision" onClick={(e) => e.stopPropagation()}>
      <div className="suggestion-head">
        <Icon name="gavel" size={12} /> {draft ? "Record what was decided?" : "What was decided"} {draft && <span className="dim">drafted by {draft.by === "agent" ? "the agent" : draft.by}</span>}
      </div>
      <textarea value={what} rows={Math.min(5, Math.max(2, Math.ceil(what.length / 40)))} placeholder="The decision, in a sentence" onChange={(e) => setWhat(e.target.value)} onKeyDown={keys} />
      <textarea value={why} rows={Math.min(5, Math.max(2, Math.ceil(why.length / 40)))} placeholder="Why" onChange={(e) => setWhy(e.target.value)} onKeyDown={keys} />
      <div className="note-actions">
        <span className="spacer-left" />
        {draft ? (
          <button className="ghost small" disabled={busy} onClick={() => run({ action: "skip" })}>
            Skip
          </button>
        ) : (
          <button className="ghost small" onClick={onCancel}>
            Cancel
          </button>
        )}
        <button className="primary small" disabled={busy || !what.trim()} onClick={() => run({ action: "record", what, why })} title="Record it (⌘⏎)">
          Record
        </button>
      </div>
    </div>
  );
}

// Composer writes a new thread on what's highlighted: tab picks ask,
// change or note; ⌘⏎ posts it, and it's live at once.
function Composer({ pick, onPost, onCancel }: { pick: Pick; onPost: (kind: ThreadKind, body: string, suggestion?: string) => Promise<void>; onCancel: () => void }) {
  const [kind, setKind] = useState<ThreadKind>("ask");
  const [text, setText] = useState("");
  // An edit of your own: what the highlighted text should read.
  const [edit, setEdit] = useState<string>();
  const [error, setError] = useState<string>();
  const area = useRef<HTMLTextAreaElement>(null);
  useAutosize(area, text, 260);
  // While writing, the page's keys are off: these are the composer's. (Its
  // slot is see-through, not hidden, until it's placed: focus needs that.)
  useKeys({}, { modal: true });
  useEffect(() => {
    area.current?.focus({ preventScroll: true });
  }, []);
  const post = () => {
    if (edit !== undefined) {
      if (edit === pick.quote.exact) return;
      onPost("change", text.trim() || "This should read differently.", edit).catch((e: Error) => setError(e.message));
      return;
    }
    if (!text.trim()) return;
    onPost(kind, text).catch((e: Error) => setError(e.message));
  };
  const suggest = () => {
    setKind("change");
    setEdit(pick.quote.exact);
  };
  return (
    <div className="note-editor doc-note" onClick={(e) => e.stopPropagation()}>
      <div className="segmented kinds" role="radiogroup" aria-label="What kind of comment">
        {threadKinds.map((k) => (
          <button key={k} role="radio" aria-checked={k === kind} aria-pressed={k === kind} className={`k-${k}`} onClick={() => setKind(k)} title={`${kindWords[k]} (tab cycles)`}>
            <Icon name={kindIcon[k]} size={12} />
            {kindWords[k]}
          </button>
        ))}
      </div>
      <blockquote className="editor-on">{pick.quote.exact ? clip(pick.quote.exact.replace(/\s+/g, " "), 140) : "The whole diagram"}</blockquote>
      <textarea
        ref={area}
        value={text}
        rows={2}
        placeholder={kindHints[kind]}
        aria-label={kindHints[kind]}
        onChange={(e) => setText(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
            e.preventDefault();
            post();
          } else if (e.key === "Escape") {
            e.preventDefault();
            onCancel();
          } else if (e.key === "Tab") {
            e.preventDefault();
            const i = threadKinds.indexOf(kind);
            setKind(threadKinds[(i + (e.shiftKey ? threadKinds.length - 1 : 1)) % threadKinds.length]);
          }
        }}
      />
      {edit !== undefined && (
        <>
          <div className="dim editor-on">It should read:</div>
          <textarea
            className="edit-to"
            value={edit}
            rows={Math.min(10, Math.max(2, edit.split("\n").length + 1))}
            onChange={(e) => setEdit(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
                e.preventDefault();
                post();
              } else if (e.key === "Escape") {
                e.preventDefault();
                onCancel();
              }
            }}
          />
        </>
      )}
      {error && <div className="banner">{humanError(error)}</div>}
      {pick.quote.exact && edit === undefined && (
        <button className="link suggest-link" onClick={suggest}>
          <Icon name="pencil" size={12} /> Suggest an edit instead
        </button>
      )}
      <div className="note-actions">
        <span className="dim composer-tip">
          <Keys k="tab" /> kind
        </span>
        <span className="spacer-left" />
        <button className="ghost small" onClick={onCancel} title="Cancel (esc)">
          Cancel
        </button>
        <button className="primary small" onClick={post} disabled={edit !== undefined ? edit === pick.quote.exact : !text.trim()} title="Post it (⌘⏎)">
          {edit !== undefined ? "Suggest" : "Comment"}
        </button>
      </div>
    </div>
  );
}

// ChangePop is what a changed passage said when you last commented.
function ChangePop({ top, old, added, onClose }: { top: number; old: string[]; added: boolean; onClose: () => void }) {
  const text = old.join("\n").trim();
  return (
    <div className="change-pop" style={{ top }} onClick={(e) => e.stopPropagation()}>
      <div className="note-head">
        <span className="dim">{text ? "When you last commented, this read:" : "Added since you last commented."}</span>
        <span className="spacer-left" />
        <button className="ghost icon-button small" onClick={onClose} title="Close (esc)" aria-label="Close">
          <Icon name="x" size={12} />
        </button>
      </div>
      {text && <pre className="was">{text}</pre>}
      {!added && <div className="dim">Removed since.</div>}
    </div>
  );
}
