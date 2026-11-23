import { type ReactNode, useLayoutEffect, useRef, useState } from "react";
import { useOverlayPosition } from "react-aria";
import { createPortal } from "react-dom";
import { NoRefs } from "../refs/known";
import { cx } from "./cx";
import css from "./FloatingCard.module.css";

// Anchor is what a card is placed by: an element, or anything else that
// says where it is in the window and whether it's still there, as an
// element does (words in a plugin's frame: plugins/ui.ts).
export interface Anchor {
  readonly isConnected: boolean;
  getBoundingClientRect(): { top: number; left: number; bottom: number; right: number };
}

// How far a card keeps from the window's edge, from what it's under, and
// from the side of a list it's beside.
const edge = 12;
const under = 6;
const aside = 10;

// FloatingCard is a hover card's panel: under what it's about (anchor), or
// over it where there's no room below, kept on screen, and placed again as
// it grows (what it shows arrives). Given beside (a list the anchor is in,
// say), it goes at that's side instead, level with the anchor, so it
// doesn't cover what's next in the list: left of it, or right where
// there's no room on the left; under the anchor, as ever, where there's
// room on neither.
//
// Where it goes is react-aria's to work out (useOverlayPosition), from a
// rectangle it's told: the anchor's, which needn't be an element's, or
// the anchor's height at the list's sides.
export function FloatingCard({
  anchor,
  beside,
  className,
  onEnter,
  onLeave,
  children,
}: {
  anchor: Anchor;
  beside?: HTMLElement | null;
  className?: string;
  onEnter: () => void;
  onLeave: () => void;
  children: ReactNode;
}) {
  const card = useRef<HTMLDivElement>(null);
  // react-aria is given an element to watch: the anchor where it's one,
  // the page where it isn't. What it places by is the rectangle below.
  const target = useRef<Element>(document.documentElement);
  target.current = anchor instanceof Element ? anchor : document.documentElement;
  // At the list's side, unless there's room on neither: known once the
  // card's width is.
  const [noSide, setNoSide] = useState(false);
  const atSide = !!beside && !noSide;
  useLayoutEffect(() => {
    if (!beside?.isConnected || !card.current) return setNoSide(false);
    const list = beside.getBoundingClientRect();
    const w = card.current.offsetWidth;
    const fits = (x: number) => x >= edge && x + w <= window.innerWidth - edge;
    setNoSide(!fits(list.left - aside - w) && !fits(list.right + aside));
  }, [anchor, beside]);
  const { overlayProps } = useOverlayPosition({
    targetRef: target,
    overlayRef: card,
    isOpen: true,
    placement: atSide ? "left top" : "bottom start",
    offset: atSide ? aside : under,
    crossOffset: atSide ? -aside : -under,
    containerPadding: edge,
    shouldFlip: true,
    getTargetRect: () => {
      if (!anchor.isConnected) return null;
      const at = anchor.getBoundingClientRect();
      if (!atSide || !beside?.isConnected) return new DOMRect(at.left, at.top, at.right - at.left, at.bottom - at.top);
      const list = beside.getBoundingClientRect();
      return new DOMRect(list.left, at.top, list.width, at.bottom - at.top);
    },
  });
  return createPortal(
    <div
      {...overlayProps}
      ref={card}
      className={cx(css.card, className)}
      role="tooltip"
      // A dialog makes the rest of the page inert: not a card over it.
      data-react-aria-top-layer=""
      data-hover-card=""
      onMouseEnter={onEnter}
      onMouseLeave={onLeave}
    >
      <NoRefs>{children}</NoRefs>
    </div>,
    document.body,
  );
}
