import { useEffect, useLayoutEffect, useRef, useState, useSyncExternalStore } from "react";
import { useKeys } from "../keys";
import css from "./Viewer.module.css";
import { cx } from "./cx";
import { Icon } from "./Icon";
import { Keys } from "./Keys";
import { Button, buttonClass } from "./Button";

// The viewer: a picture (an image, an SVG, a Mermaid diagram) over the
// whole page, to look around in. Drag to move it, the wheel (or a pinch)
// to zoom in on where the pointer is, esc to close. It eases to where you
// send it rather than jumping, but follows a drag exactly.
//
// Anything drawn with data-viewable opens it on a click, wherever it is
// in the app: an <img>, or an element with an <svg> inside (a Mermaid
// diagram's). One inside a link is the link's. view() opens it from code.

export type Viewing = { kind: "image"; src: string; title?: string } | { kind: "svg"; svg: string; title?: string };

let current: Viewing | null = null;
const listeners = new Set<() => void>();

export function view(v: Viewing | null) {
  current = v;
  listeners.forEach((l) => l());
}

function useViewing(): Viewing | null {
  return useSyncExternalStore(
    (l) => {
      listeners.add(l);
      return () => listeners.delete(l);
    },
    () => current,
  );
}

// viewingOf is what a click on el opens, if it opens anything.
export function viewingOf(el: Element): Viewing | null {
  const at = el.closest("[data-viewable]");
  if (!at || el.closest("a")) return null;
  const title = at.getAttribute("data-viewable") || at.getAttribute("alt") || at.getAttribute("title") || undefined;
  if (at instanceof HTMLImageElement) return at.currentSrc || at.src ? { kind: "image", src: at.currentSrc || at.src, title } : null;
  const svg = at.querySelector("svg");
  return svg ? { kind: "svg", svg: svg.outerHTML, title } : null;
}

// Where the picture is: moved by (x, y) and scaled by s, from its top left.
interface At {
  x: number;
  y: number;
  s: number;
}

const most = 32; // times its own size
const step = 1.5; // + and -

export function Viewer() {
  const viewing = useViewing();
  // Clicks on what's viewable, anywhere.
  useEffect(() => {
    const onClick = (e: MouseEvent) => {
      if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey) return;
      const v = e.target instanceof Element ? viewingOf(e.target) : null;
      if (!v) return;
      e.preventDefault();
      view(v);
    };
    document.addEventListener("click", onClick);
    return () => document.removeEventListener("click", onClick);
  }, []);
  if (!viewing) return null;
  return <Stage key={viewing.kind === "image" ? viewing.src : viewing.svg} viewing={viewing} />;
}

function Stage({ viewing }: { viewing: Viewing }) {
  const stage = useRef<HTMLDivElement>(null);
  const body = useRef<HTMLDivElement>(null);
  // Its own size, once it's known (an image's once it's loaded).
  const [size, setSize] = useState<{ w: number; h: number }>();
  const [failed, setFailed] = useState(false);
  const [zoom, setZoom] = useState(1);
  const [closing, setClosing] = useState(false);

  // Where it is (now) and where it's going (to): each frame, now eases
  // towards to. A drag moves both.
  const now = useRef<At>({ x: 0, y: 0, s: 1 });
  const to = useRef<At>({ x: 0, y: 0, s: 1 });
  const fit = useRef(1);
  const frame = useRef(0);
  const vector = viewing.kind === "svg" || /\.svg($|\?)/i.test(viewing.src) || viewing.src.startsWith("data:image/svg");

  const paint = () => {
    const el = body.current;
    if (!el) return;
    const { x, y, s } = now.current;
    el.style.transform = `translate3d(${x}px, ${y}px, 0) scale(${s})`;
    // A photo's pixels show as pixels close up; an SVG's redrawn sharp.
    el.toggleAttribute("data-pixels", !vector && s >= 3);
  };
  const run = () => {
    if (frame.current) return;
    let last = performance.now();
    const tick = (t: number) => {
      const k = 1 - Math.exp(-(t - last) / 70);
      last = t;
      const a = now.current;
      const b = to.current;
      a.x += (b.x - a.x) * k;
      a.y += (b.y - a.y) * k;
      a.s += (b.s - a.s) * k;
      const done = Math.abs(b.x - a.x) < 0.3 && Math.abs(b.y - a.y) < 0.3 && Math.abs(b.s - a.s) < b.s * 0.001;
      if (done) Object.assign(a, b);
      paint();
      setZoom(a.s);
      frame.current = done ? 0 : requestAnimationFrame(tick);
    };
    frame.current = requestAnimationFrame(tick);
  };
  const jump = (at: At) => {
    to.current = { ...at };
    now.current = { ...at };
    paint();
    setZoom(at.s);
  };
  useEffect(() => () => cancelAnimationFrame(frame.current), []);

  const bounds = () => stage.current?.getBoundingClientRect() ?? new DOMRect(0, 0, innerWidth, innerHeight);
  // centred is it at scale s, in the middle of the stage.
  const centred = (s: number): At => {
    const r = bounds();
    if (!size) return { x: 0, y: 0, s };
    return { x: (r.width - size.w * s) / 2, y: (r.height - size.h * s) / 2, s };
  };
  const clamp = (s: number) => Math.min(most, Math.max(Math.min(fit.current, 1) * 0.2, s));
  // zoomAt scales to s, keeping the point (px, py) of the stage where it is.
  const zoomAt = (s: number, px: number, py: number, ease = true) => {
    const b = to.current;
    const n = clamp(s);
    const next = { x: px - ((px - b.x) * n) / b.s, y: py - ((py - b.y) * n) / b.s, s: n };
    if (ease) {
      to.current = next;
      run();
    } else jump(next);
  };
  const middle = () => {
    const r = bounds();
    return [r.width / 2, r.height / 2] as const;
  };
  const toFit = () => {
    to.current = centred(fit.current);
    run();
  };
  const toActual = () => {
    const [px, py] = middle();
    zoomAt(1, px, py);
  };

  // Once its size is known: fitted to the stage, a little in from its
  // edges. A small picture isn't blown up past its own size, unless it's
  // drawn (an SVG), which stays sharp: up to 4 times.
  useLayoutEffect(() => {
    if (!size) return;
    const r = bounds();
    fit.current = Math.min((r.width - 96) / size.w, (r.height - 112) / size.h, vector ? 4 : 1);
    jump(centred(fit.current));
    // Fitted again as the window's resized, if it was fitted.
    const onResize = () => {
      const was = fit.current;
      const r2 = bounds();
      fit.current = Math.min((r2.width - 96) / size.w, (r2.height - 112) / size.h, vector ? 4 : 1);
      if (Math.abs(to.current.s - was) < was * 0.01) jump(centred(fit.current));
    };
    addEventListener("resize", onResize);
    return () => removeEventListener("resize", onResize);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [size]);

  // An SVG's size: its width and height, or its viewBox's. Mermaid's say
  // 100% and cap it with max-width: drawn at the viewBox's size instead.
  useLayoutEffect(() => {
    if (viewing.kind !== "svg") return;
    const svg = body.current?.querySelector("svg");
    if (!svg) return;
    const vb = svg.viewBox?.baseVal;
    const attr = (n: string) => {
      const v = svg.getAttribute(n);
      return v && /^[\d.]+(px)?$/.test(v) ? parseFloat(v) : 0;
    };
    const w = attr("width") || vb?.width || svg.getBoundingClientRect().width || 800;
    const h = attr("height") || vb?.height || svg.getBoundingClientRect().height || 600;
    svg.setAttribute("width", String(w));
    svg.setAttribute("height", String(h));
    svg.style.maxWidth = "none";
    setSize({ w, h });
  }, [viewing]);

  // The wheel zooms (a pinch is a wheel with ctrl held, finer).
  useEffect(() => {
    const el = stage.current;
    if (!el) return;
    const onWheel = (e: WheelEvent) => {
      e.preventDefault();
      const r = el.getBoundingClientRect();
      const lines = e.deltaMode === 1 ? 16 : e.deltaMode === 2 ? r.height : 1;
      const dy = Math.max(-120, Math.min(120, e.deltaY * lines));
      const factor = Math.exp(-dy * (e.ctrlKey ? 0.012 : 0.0025));
      zoomAt(to.current.s * factor, e.clientX - r.left, e.clientY - r.top);
    };
    el.addEventListener("wheel", onWheel, { passive: false });
    return () => el.removeEventListener("wheel", onWheel);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [size]);

  // A drag moves it, exactly: no easing behind the pointer. Let go while
  // it's moving and it glides a little further.
  const drag = useRef<{ id: number; x: number; y: number; vx: number; vy: number; t: number; moved: number } | null>(null);
  const [dragging, setDragging] = useState(false);
  const onPointerDown = (e: React.PointerEvent) => {
    if (e.button !== 0 || (e.target as Element).closest("[data-viewer-own]")) return;
    e.preventDefault();
    stage.current?.setPointerCapture(e.pointerId);
    cancelAnimationFrame(frame.current);
    frame.current = 0;
    to.current = { ...now.current };
    drag.current = { id: e.pointerId, x: e.clientX, y: e.clientY, vx: 0, vy: 0, t: performance.now(), moved: 0 };
    setDragging(true);
  };
  const onPointerMove = (e: React.PointerEvent) => {
    const d = drag.current;
    if (!d || d.id !== e.pointerId) return;
    const dx = e.clientX - d.x;
    const dy = e.clientY - d.y;
    const t = performance.now();
    const dt = Math.max(1, t - d.t);
    d.vx = d.vx * 0.6 + (dx / dt) * 0.4;
    d.vy = d.vy * 0.6 + (dy / dt) * 0.4;
    Object.assign(d, { x: e.clientX, y: e.clientY, t, moved: d.moved + Math.abs(dx) + Math.abs(dy) });
    now.current.x += dx;
    now.current.y += dy;
    to.current = { ...now.current };
    paint();
  };
  const onPointerUp = (e: React.PointerEvent) => {
    const d = drag.current;
    if (!d || d.id !== e.pointerId) return;
    drag.current = null;
    setDragging(false);
    // A click (not a drag) beside the picture closes it.
    if (e.type === "pointerup" && d.moved < 4 && !onPicture(e.clientX, e.clientY)) return close();
    // Still moving as it's let go (not held still first): on a little.
    if (performance.now() - d.t < 60) {
      to.current = { ...now.current, x: now.current.x + d.vx * 160, y: now.current.y + d.vy * 160 };
      run();
    }
  };
  const onPicture = (x: number, y: number) => {
    const r = body.current?.getBoundingClientRect();
    return !!r && x >= r.left && x <= r.right && y >= r.top && y <= r.bottom;
  };
  // A double click zooms in on where it is, or (zoomed in already) back
  // to fitted.
  const onDoubleClick = (e: React.MouseEvent) => {
    if ((e.target as Element).closest("[data-viewer-own]")) return;
    if (to.current.s > fit.current * 1.05) return toFit();
    const r = bounds();
    zoomAt(Math.max(fit.current * 2.5, 1), e.clientX - r.left, e.clientY - r.top);
  };

  const close = () => {
    if (closing) return;
    setClosing(true);
    setTimeout(() => view(null), 140);
  };
  const nudge = (dx: number, dy: number) => {
    to.current = { ...to.current, x: to.current.x + dx, y: to.current.y + dy };
    run();
  };
  useKeys(
    {
      Escape: close,
      "+": () => zoomAt(to.current.s * step, ...middle()),
      "=": () => zoomAt(to.current.s * step, ...middle()),
      "-": () => zoomAt(to.current.s / step, ...middle()),
      "0": toFit,
      "1": toActual,
      ArrowLeft: () => nudge(80, 0),
      ArrowRight: () => nudge(-80, 0),
      ArrowUp: () => nudge(0, 80),
      ArrowDown: () => nudge(0, -80),
      h: () => nudge(80, 0),
      l: () => nudge(-80, 0),
      k: () => nudge(0, 80),
      j: () => nudge(0, -80),
    },
    { modal: true },
  );

  return (
    <div
      className={cx(css.viewer, dragging && css.dragging, closing && css.closing)}
      role="dialog"
      aria-modal="true"
      aria-label={viewing.title ?? "Picture"}
      ref={stage}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onPointerCancel={onPointerUp}
      onDoubleClick={onDoubleClick}
    >
      <div className={cx(css.body, !size && css.unsized)} ref={body}>
        {viewing.kind === "image" ? (
          <img
            src={viewing.src}
            alt={viewing.title ?? ""}
            draggable={false}
            onLoad={(e) => {
              const img = e.currentTarget;
              // An SVG without a size of its own has none to read.
              setSize({ w: img.naturalWidth || 800, h: img.naturalHeight || 600 });
            }}
            onError={() => setFailed(true)}
          />
        ) : (
          <div className={css.svg} dangerouslySetInnerHTML={{ __html: viewing.svg }} />
        )}
      </div>
      {failed && <div className={css.failed}>This picture couldn't be loaded.</div>}
      {viewing.title && (
        <div className={css.title} data-viewer-own="">
          {viewing.title}
        </div>
      )}
      <div className={css.tools} data-viewer-own="" onPointerDown={(e) => e.stopPropagation()}>
        <Button variant="ghost" square onClick={() => zoomAt(to.current.s / step, ...middle())} title="Zoom out (-)" aria-label="Zoom out">
          <Icon name="minus" size={15} />
        </Button>
        <Button variant="ghost" className={css.zoom} onClick={toActual} title="Its own size (1)">
          {Math.round(zoom * 100)}%
        </Button>
        <Button variant="ghost" square onClick={() => zoomAt(to.current.s * step, ...middle())} title="Zoom in (+)" aria-label="Zoom in">
          <Icon name="plus" size={15} />
        </Button>
        <Button variant="ghost" className={css.fit} onClick={toFit} title="Fit it to the window (0)">
          Fit
        </Button>
        {viewing.kind === "image" && (
          <a className={buttonClass({ variant: "ghost" }, "icon-button")} href={viewing.src} target="_blank" rel="noopener noreferrer" title="Open it on its own" aria-label="Open it on its own">
            <Icon name="external" size={14} />
          </a>
        )}
        <span className={css.sep} />
        <Button variant="ghost" square onClick={close} title="Close (esc)" aria-label="Close">
          <Icon name="x" size={15} />
        </Button>
      </div>
      <div className={css.hint} aria-hidden>
        Drag to move · scroll to zoom · <Keys k="0" /> fit · <Keys k="esc" /> close
      </div>
    </div>
  );
}
