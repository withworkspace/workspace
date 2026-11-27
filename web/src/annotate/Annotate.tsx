import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import {
  act,
  approveDoc,
  type ArtifactContent,
  draftWaiting,
  getThreads,
  getThreadVersion,
  openThread,
  pendingSuggestion,
  placeOf,
  recordDecision,
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
import { useCommands } from "../keymap/useCommands";
import { annotateKeys } from "./keys";
import { type TrackedChange, TrackedDoc } from "../editor/LazyTracked";
import { Markdown } from "../markdown/LazyMarkdown";
import { Banner } from "../ui/Banner";
import { Icon } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import { useMedia } from "../ui/useMedia";
import { toast } from "../ui/toast";
import { Callout } from "../ui/Callout";
import { Keys } from "../ui/Keys";
import { exact, stamp } from "../ui/when";
import { BLOCK, blockLike, blockLines, blockSpan, blockWithLines, buildIndex, find, offsetOf, rangeOf, type TextIndex, targetOf, targetOfRange } from "./anchor";
import { changes } from "./diff";
import { useLatest, useLoaded } from "../load";
import { Chip } from "../ui/Chip";
import { Button } from "../ui/Button";
import { humanError } from "../ui/strings";
import { pageScroller } from "../ui/scroller";
import { AnnotateAddComment } from "./AnnotateAddComment";
import { AnnotateBar, AnnotateBarNote } from "./AnnotateBar";
import { AnnotateBlockMark } from "./AnnotateBlockMark";
import { AnnotateChangeBar } from "./AnnotateChangeBar";
import { AnnotateChangePop } from "./AnnotateChangePop";
import { AnnotateCursor } from "./AnnotateCursor";
import { AnnotateDocCol, AnnotateLayout, annotateLandedClass, AnnotateMargin, AnnotateSlot, AnnotateText } from "./AnnotateLayout";
import { AnnotateLost } from "./AnnotateLost";
import { AnnotateMarginHint } from "./AnnotateMarginHint";
import { AnnotatePreviewBar } from "./AnnotatePreviewBar";
import { AnnotateSheet } from "./AnnotateSheet";
import { AnnotateThreadMarker } from "./AnnotateThreadMarker";
import { type Box, caretAt, COMPOSER, GAP, type Geo, noGeo, num, type Pick, pickOf, type Placed, sinceYou, spotRect, stack } from "./geometry";
import { ThreadCard } from "./ThreadCard";
import { ThreadComposer } from "./ThreadComposer";
import { kindWords } from "./words";


// Commenting on a document, as in a shared doc: you read it rendered,
// highlight text and comment on it, and the comment is a thread, live at
// once. Threads sit in the margin level with what they're on. Crew answer
// in them (ws thread reply); you reply, and resolve them when you're done.
// What's changed since you last commented is marked in the left margin.
//
// Highlights are drawn with the CSS Custom Highlight API, so the rendered
// markdown (React's) is never touched: a thread is a Range over it.
//
// AnnotatedDoc is what knows and does: the threads, where they are, the
// cursor and what's picked, the keys. What it draws are the Annotate…
// parts (the bar, the layout of column and margin, what's over the
// document) and a ThreadCard for each thread.

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
    const main = pageScroller();
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
    const top = pageScroller()?.getBoundingClientRect().top ?? 0;
    const i = nav.findIndex((p) => !p.spot || (spotRect(p.spot)?.top ?? 0) >= top);
    return i >= 0 ? i : 0;
  };
  const moveCursor = (step: number, extend = false) => {
    if (!blocks.length) return;
    let to: number;
    if (!cursor) {
      const top = pageScroller()?.getBoundingClientRect().top ?? 0;
      to = Math.max(0, blocks.findIndex((b) => b.getBoundingClientRect().bottom > top + 8));
    } else to = Math.min(Math.max(cursor.to + step, 0), blocks.length - 1);
    setCursor({ from: extend && cursor ? cursor.from : to, to });
    setActive(undefined);
    const r = blocks[to].getBoundingClientRect();
    const main = pageScroller();
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
      setError(`${changes.join(", ")} ${one ? "asks" : "ask"} for a change: resolve ${one ? "it" : "them"} first.`);
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
        b.classList.add(annotateLandedClass);
        setTimeout(() => b.classList.remove(annotateLandedClass), 2400);
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
  useCommands(annotateKeys, {
    next: () => moveCursor(1),
    prev: () => moveCursor(-1),
    extendNext: () => moveCursor(1, true),
    extendPrev: () => moveCursor(-1, true),
    comment,
    nextThread: () => go(at < 0 ? firstInView() : at + 1),
    prevThread: () => go(at < 0 ? firstInView() - 1 : at - 1),
    reply: () => current && setReplying(current.thread.id),
    resolve: () => toggleResolved(current?.thread),
    accept: () => settle(preview?.thread ?? current?.thread, "accept"),
    decline: () => settle(preview?.thread ?? current?.thread, "decline"),
    since: () => since && setShowChanges((v) => !v),
    approve,
    tracked: () => {
      if (previewing) setPreviewing(undefined);
      else if (current && pendingSuggestion(current.thread)) setPreviewing(current.thread.id);
      else if (edits.length) setPreviewing(edits[0].id);
    },
    // Esc clears what's picked out first; with nothing, goes back.
    back: escape,
    backAlso: onBack,
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
    <div>
      <AnnotateBar
        counts={
          <>
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
                <Spinner size={11} /> {answering.length} answering
              </Chip>
            )}
            {drafts.length > 0 && (
              <Chip tone="attention">
                <Icon name="gavel" size={11} /> {drafts.length} {drafts.length === 1 ? "decision" : "decisions"} to record
              </Chip>
            )}
          </>
        }
      >
        {!own && toSend.length > 1 && (
          <Button
            disabled={sending || stale}
            onClick={() => sendToCrew(toSend.map((t) => t.id))}
            title={`Send ${toSend.map((t) => t.id).join(", ")} to crew`}
          >
            Send {toSend.length} changes to crew
          </Button>
        )}
        {since && diff.length > 0 && (
          <Button variant="ghost" size="small" onClick={() => setShowChanges((v) => !v)} title="Changes since your last comment" keys={annotateKeys.since}>
            <Icon name="eye" size={13} /> {showChanges ? "Hide changes" : "Show changes"} <Keys k={annotateKeys.since} />
          </Button>
        )}
        {own ? null : approvedNow ? (
          <Chip tone="done" title={`You approved this version ${exact(approved.at)}`}>
            <Icon name="check-circle" size={12} /> Approved
          </Chip>
        ) : (
          <>
            {approved && <AnnotateBarNote>You approved an earlier version</AnnotateBarNote>}
            <Button variant={open.some((t) => t.kind === "change") ? undefined : "primary"} disabled={approving || stale} onClick={approve} title="Approve this version" keys={annotateKeys.approve}>
              <Icon name="check" size={13} /> {approved ? "Approve again" : "Approve"} <Keys k={annotateKeys.approve} />
            </Button>
          </>
        )}
      </AnnotateBar>
      {offerStage && stage && (
        <Callout
          tone="done"
          title="Approved"
          actions={
            <>
              <Button variant="primary" onClick={acceptStage}>
                Accept stage {stage.n}
              </Button>
              <Button variant="ghost" onClick={() => setOfferDismissed(true)}>
                Not now
              </Button>
            </>
          }
        >
          This completes stage {stage.n} ({stage.title}). Accept the stage too?
        </Callout>
      )}
      {stale && (
        <Callout
          tone="attention"
          title="Document updated"
          actions={
            <Button onClick={onChanged} keys="r">
              Reload <Keys k="r" />
            </Button>
          }
        >
          Reload to add new threads.
        </Callout>
      )}
      {since && diff.length > 0 && showChanges && (
        <Callout tone="info" icon="eye" title={`${diff.length} ${diff.length === 1 ? "change" : "changes"} since you commented ${stamp(since.at).replace(/^Yesterday/, "yesterday")}`}>
          Marked in the margin: click one to see the earlier text.
          {waiting.length > 0 && (
            <>
              {" "}
              {waiting.length} {waiting.length === 1 ? "thread is" : "threads are"} waiting on you: <Keys k={annotateKeys.nextThread} /> goes through them.
            </>
          )}
        </Callout>
      )}
      {error && <Banner>{humanError(error)}</Banner>}
      {lost.length > 0 && (
        <AnnotateLost said={`${lost.length === 1 ? "A thread's" : `${lost.length} threads'`} text was changed or removed in this version.`}>
          {lost.map((p) => (
            <div key={p.thread.id} id={`thread-${p.thread.id}`}>
              {card(p, true)}
            </div>
          ))}
        </AnnotateLost>
      )}
      {preview && <AnnotatePreviewBar count={edits.length} marked={preview.thread.id} />}
      {preview && (
        <AnnotateLayout>
          <AnnotateDocCol>
            <AnnotateText>
              <TrackedDoc
                text={artifact.text}
                changes={edits}
                focus={preview.thread.id}
                settling={settling}
                onFocus={setPreviewing}
                onSettle={(id, action) => settle(threads.find((x) => x.id === id), action)}
                keys={{ accept: annotateKeys.accept, decline: annotateKeys.decline }}
              />
            </AnnotateText>
          </AnnotateDocCol>
        </AnnotateLayout>
      )}
      <AnnotateLayout hidden={!!preview}>
        <AnnotateDocCol ref={docCol}>
          <AnnotateText ref={article} onClick={docClick} onMouseMove={textHover} onMouseLeave={() => hoverFromText.current && setHover(undefined)}>
            <Markdown text={artifact.text} lines at={place} />
          </AnnotateText>
          {geo.cursor && <AnnotateCursor top={geo.cursor.top} height={geo.cursor.height} />}
          {geo.marks.map((m) => (
            <AnnotateBlockMark key={m.id} on={m.id === active || m.id === COMPOSER || m.id === hover} top={m.top - 4} height={m.height + 8} left={m.left - 4} width={m.width + 8} />
          ))}
          {narrow &&
            (geo.anchors ?? []).map((a, i, all) => {
              const p = shown.find((x) => x.thread.id === a.id);
              if (!p) return null;
              const t = p.thread;
              const stacked = all.slice(0, i).filter((b) => Math.abs(b.top - a.top) < 20).length;
              return (
                <AnnotateThreadMarker
                  key={a.id}
                  kind={t.kind}
                  top={a.top + stacked * 24}
                  turns={t.turns.length}
                  resolved={t.status === "resolved"}
                  yours={waitingOnYou(t)}
                  on={a.id === active}
                  label={`${kindWords[t.kind]} thread ${t.id}${t.status === "resolved" ? ", resolved" : ""}`}
                  onPress={() => {
                    setActive(a.id);
                    setSheet(a.id);
                  }}
                />
              );
            })}
          {geo.bars.map((b) => (
            <AnnotateChangeBar key={b.i} top={b.top} height={b.height} removed={b.removed} on={openChange === b.i} onPress={() => setOpenChange((o) => (o === b.i ? undefined : b.i))} />
          ))}
          {openChange !== undefined && diff[openChange] && (
            <AnnotateChangePop
              top={(geo.bars.find((b) => b.i === openChange)?.top ?? 0) + (geo.bars.find((b) => b.i === openChange)?.height ?? 0) + 6}
              old={diff[openChange].old}
              added={diff[openChange].newEnd >= diff[openChange].newStart}
              onClose={() => setOpenChange(undefined)}
            />
          )}
        </AnnotateDocCol>
        <AnnotateMargin height={geo.height}>
          {geo.button !== undefined && !composing && !stale && <AnnotateAddComment top={geo.button} text={!!selection} onPress={comment} />}
          {shown.map((p) => (
            <AnnotateSlot key={p.thread.id} ref={setCard(p.thread.id)} top={geo.cards[p.thread.id]}>
              {card(p)}
            </AnnotateSlot>
          ))}
          {!shown.length && !composing && geo.button === undefined && !stale && <AnnotateMarginHint />}
          {composing && !narrow && (
            <AnnotateSlot key={COMPOSER} ref={setCard(COMPOSER)} top={geo.cards[COMPOSER]}>
              <ThreadComposer pick={composing} onPost={post} onCancel={() => setComposing(undefined)} />
            </AnnotateSlot>
          )}
        </AnnotateMargin>
      </AnnotateLayout>
      {narrow && (composing || (sheet && shown.some((p) => p.thread.id === sheet))) && (
        <AnnotateSheet
          label={composing ? "New comment" : "Thread"}
          onClose={() => {
            setSheet(undefined);
            setComposing(undefined);
          }}
        >
          {composing ? <ThreadComposer pick={composing} onPost={post} onCancel={() => setComposing(undefined)} /> : card(shown.find((p) => p.thread.id === sheet)!)}
        </AnnotateSheet>
      )}
    </div>
  );
}
