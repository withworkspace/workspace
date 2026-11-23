import { type ReactNode, useLayoutEffect, useRef, useState } from "react";
import css from "./Clamp.module.css";
import { cx } from "./cx";
import { MoreLink } from "./MoreLink";

type ClampProps = {
  children: ReactNode;
  // How much shows before the rest is asked for: a height in em, faded
  // out at its foot (the default, 9.5), or a count of lines, the last
  // ending in an ellipsis.
  tall?: number;
  lines?: number;
  // What it's of (its text): when that changes it's measured again,
  // and closed.
  of?: unknown;
  // The words that open it, and close it.
  more?: string;
  less?: string;
  // The page's own class, to place it.
  className?: string;
};

// Clamp is something long, cut short until the rest is asked for: a few
// lines of what crew said, a summary. The way to the rest is there only
// when there is more.
export function Clamp({ children, tall = 9.5, lines, of, more = "Show all", less = "Show less", className }: ClampProps) {
  const body = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  const [long, setLong] = useState(false);
  useLayoutEffect(() => {
    setOpen(false);
    const el = body.current;
    if (!el) return;
    // Measured closed, whatever it was: this runs before it's painted.
    el.dataset.open = "false";
    setLong(el.scrollHeight > el.clientHeight + 4);
  }, [of, tall, lines]);
  return (
    <div className={className} data-clamp="">
      <div
        ref={body}
        data-open={open}
        data-long={long}
        className={cx(css.body, lines ? css.lines : css.tall)}
        style={{ [lines ? "--clamp-lines" : "--clamp-tall"]: lines ?? `${tall}em` } as React.CSSProperties}
      >
        {children}
      </div>
      {(long || open) && (
        <MoreLink onClick={() => setOpen((o) => !o)} aria-expanded={open}>
          {open ? less : more}
        </MoreLink>
      )}
    </div>
  );
}
