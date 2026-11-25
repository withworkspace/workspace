import { type CSSProperties, type ReactNode, useCallback, useLayoutEffect, useMemo, useRef, useState } from "react";
import { CodeLine } from "./CodeLine";
import css from "./CodeLines.module.css";
import { type CodeFold, type CodeMark, type CodeRange, opened, rangeFrom } from "./codeLayout";
import { cx } from "./cx";

// How many lines are in a run: what's laid out together, or not at all.
const RUN = 200;

// CodeLines is a file's lines to read: numbered, highlighted, the changed
// ones marked beside their numbers. It's one file at one commit: the
// explorer's file, a definition peeked at beside a review. It draws what
// it's given and reads nothing: html is the lines highlighted or escaped
// (review/highlight), and what's known of the file is the page's to say.
//
//   marks    how each line's changed since the file's base (marksOf)
//   picked   the lines picked, tinted; onPick is told when a number's
//            pressed (that line), shift-pressed (from the first picked to
//            it) or dragged along (the run)
//   folds    what folds away, each under the line it starts at; what's
//            folded is the page's (folded, onFold) or its own
//   after    what's drawn after a line, across the code's width: a
//            comment on it. It stays in view when the code's scrolled
//   reveal   a line to bring to the middle and flash, when it changes
//            (a definition gone to); a fold of its own over it opens
//   wrap     long lines wrap, where they'd otherwise scroll sideways
//
// rev, path and side go on each line's code, for what follows names in it
// (review/CodeNav reads them). It scrolls sideways in itself; down, it's
// as tall as its lines unless the page's class gives it a height, and
// only the lines near what's in view are laid out either way.
export function CodeLines({
  html,
  path,
  rev,
  side = "new",
  marks,
  picked,
  onPick,
  folds,
  folded,
  onFold,
  after,
  reveal,
  wrap = false,
  className,
  "aria-label": label,
}: {
  html: string[];
  path: string;
  rev?: string;
  side?: "old" | "new";
  marks?: ReadonlyMap<number, CodeMark>;
  picked?: CodeRange;
  onPick?: (lines: CodeRange) => void;
  folds?: CodeFold[];
  folded?: ReadonlySet<number>;
  onFold?: (folded: Set<number>) => void;
  after?: (line: number) => ReactNode;
  reveal?: number;
  wrap?: boolean;
  className?: string;
  "aria-label"?: string;
}) {
  const box = useRef<HTMLDivElement>(null);
  const foldAt = useMemo(() => new Map((folds ?? []).map((f) => [f.at, f])), [folds]);

  // What's folded: the page's, or its own.
  const [own, setOwn] = useState<ReadonlySet<number>>(new Set());
  const shutNow = folded ?? own;
  const latest = useRef({ shutNow, folded, onFold, picked, onPick });
  latest.current = { shutNow, folded, onFold, picked, onPick };
  const setShut = (next: Set<number>) => {
    const l = latest.current;
    if (l.folded === undefined) setOwn(next);
    l.onFold?.(next);
  };
  const toggle = useCallback((at: number) => {
    const next = new Set(latest.current.shutNow);
    if (!next.delete(at)) next.add(at);
    setShut(next);
  }, []);

  // A line gone to: its own fold over it opens, then it's brought to the
  // middle and flashed, once for each time it's asked for.
  const hidden = reveal !== undefined && folded === undefined && opened(folds ?? [], own, [reveal]) !== own;
  if (hidden) setOwn(opened(folds ?? [], own, [reveal!]));
  const revealed = useRef<string>(undefined);
  useLayoutEffect(() => {
    const key = `${path}:${reveal}`;
    if (reveal === undefined || hidden || revealed.current === key) return;
    const el = box.current?.querySelector<HTMLElement>(`[data-line="${reveal}"]`);
    if (!el) return;
    revealed.current = key;
    el.scrollIntoView({ block: "center" });
    el.removeAttribute("data-arrived");
    void el.offsetWidth;
    el.setAttribute("data-arrived", "");
  }, [path, reveal, hidden, html]);

  // How wide the box is, for what's drawn after a line: it's that wide,
  // however wide the longest line makes what scrolls in it. (Measured,
  // not asked of the box in CSS: a container's width costs every line a
  // layout, which a long file can't pay.)
  const slots = !!after;
  useLayoutEffect(() => {
    const el = box.current;
    if (!el || !slots || typeof ResizeObserver === "undefined") return;
    const measure = () => el.style.setProperty("--code-width", `${el.clientWidth}px`);
    measure();
    const seen = new ResizeObserver(measure);
    seen.observe(el);
    return () => seen.disconnect();
  }, [slots]);

  // Picking: a number pressed is its line, with shift it's from the first
  // line picked to it, and dragged along the numbers it's the run.
  const dragging = useRef<number>(undefined);
  const lineAt = (e: React.PointerEvent) => {
    const el = document.elementFromPoint(e.clientX, e.clientY)?.closest<HTMLElement>("[data-line]");
    return el && box.current?.contains(el) ? Number(el.dataset.line) : undefined;
  };
  const down = (e: React.PointerEvent) => {
    const { picked: was, onPick: pick } = latest.current;
    const number = (e.target as HTMLElement).closest<HTMLElement>("[data-number]");
    if (!pick || !number || e.button !== 0) return;
    const n = Number(number.dataset.number);
    const anchor = e.shiftKey && was ? was.from : n;
    dragging.current = anchor;
    pick(rangeFrom(anchor, n));
    e.preventDefault();
  };
  const move = (e: React.PointerEvent) => {
    if (dragging.current === undefined) return;
    const n = lineAt(e);
    const { picked: was, onPick: pick } = latest.current;
    if (n === undefined || !pick) return;
    const next = rangeFrom(dragging.current, n);
    if (next.from !== was?.from || next.to !== was?.to) pick(next);
  };
  const up = () => {
    dragging.current = undefined;
  };

  // The lines in runs of RUN, by their numbers, so a run is the same run
  // whatever's folded: each is laid out only when it's near what's in view.
  const runs: ReactNode[] = [];
  let run: ReactNode[] = [];
  let hiddenTo = 0;
  const close = (first: number) => {
    if (run.length) {
      runs.push(
        <div key={first} className={css.run} style={{ "--code-run": run.length } as CSSProperties}>
          {run}
        </div>,
      );
    }
    run = [];
  };
  html.forEach((h, i) => {
    const n = i + 1;
    if (i > 0 && i % RUN === 0) close(n - RUN);
    if (n <= hiddenTo) return;
    const f = foldAt.get(n);
    const shut = !!f && shutNow.has(n);
    if (shut) hiddenTo = Math.max(hiddenTo, f.to);
    run.push(
      <CodeLine
        key={n}
        n={n}
        html={h}
        path={path}
        rev={rev}
        side={side}
        mark={marks?.get(n)}
        picked={!!picked && n >= picked.from && n <= picked.to}
        folds={!!folds?.length}
        fold={f?.to}
        shut={shut}
        onFold={toggle}
        after={after?.(n)}
      />,
    );
  });
  close(Math.floor((html.length - 1) / RUN) * RUN + 1);

  return (
    <div
      ref={box}
      role="group"
      aria-label={label ?? path}
      className={cx(css.lines, wrap && css.wrap, className)}
      style={{ "--code-digits": Math.max(2, String(html.length).length) } as CSSProperties}
      onPointerDown={down}
      onPointerMove={move}
      onPointerUp={up}
      onPointerLeave={up}
    >
      {runs}
    </div>
  );
}
