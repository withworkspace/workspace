import { LIST_MIN, PREVIEW_MIN } from "./useListWidth";
import css from "./SpaceEdge.module.css";

// SpaceEdge is the edge between a space's list and its preview, in a
// SpaceSplit: dragged, or ←/→ on it, to make the list wider or narrower;
// a double click puts it back (width undefined).
export function SpaceEdge({ width, setWidth }: { width?: number; setWidth: (w: number | undefined) => void }) {
  const clamp = (split: HTMLElement, w: number) => Math.round(Math.min(Math.max(w, LIST_MIN), split.getBoundingClientRect().width - PREVIEW_MIN - 24));
  const listOf = (edge: HTMLElement) => edge.parentElement!.querySelector<HTMLElement>("[data-space-list]")!;
  const drag = (e: React.PointerEvent<HTMLDivElement>) => {
    e.preventDefault();
    const edge = e.currentTarget;
    const split = edge.parentElement!;
    const left = split.getBoundingClientRect().left;
    edge.setPointerCapture(e.pointerId);
    edge.classList.add(css.dragging);
    let w: number | undefined;
    const move = (ev: PointerEvent) => {
      w = clamp(split, ev.clientX - left - 12);
      split.style.setProperty("--space-list-w", `${w}px`);
    };
    const up = () => {
      edge.removeEventListener("pointermove", move);
      edge.classList.remove(css.dragging);
      if (w) setWidth(w);
    };
    edge.addEventListener("pointermove", move);
    edge.addEventListener("pointerup", up, { once: true });
    edge.addEventListener("pointercancel", up, { once: true });
  };
  const key = (e: React.KeyboardEvent<HTMLDivElement>) => {
    if (e.key !== "ArrowLeft" && e.key !== "ArrowRight") return;
    e.preventDefault();
    e.stopPropagation();
    const now = width ?? listOf(e.currentTarget).getBoundingClientRect().width;
    setWidth(clamp(e.currentTarget.parentElement!, now + (e.key === "ArrowRight" ? 24 : -24)));
  };
  return (
    <div
      className={css.edge}
      role="separator"
      aria-orientation="vertical"
      aria-label="The list's width"
      tabIndex={0}
      title="Drag to make the list wider or narrower; double-click to put it back"
      onPointerDown={drag}
      onDoubleClick={() => {
        setWidth(undefined);
      }}
      onKeyDown={key}
    />
  );
}
