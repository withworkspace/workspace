import type { ReactElement, ReactNode, RefObject } from "react";
import { Popover as AriaPopover, type PopoverProps as AriaPopoverProps, Dialog, DialogTrigger } from "react-aria-components";
import { cx } from "./cx";
import css from "./Popover.module.css";

// Where it opens, about what it opens from: under it at its start, unless
// it's said.
export type Placement = AriaPopoverProps["placement"];

type SurfaceProps = {
  placement?: Placement;
  offset?: number;
  maxHeight?: number;
  // The page's keys go on working while it's open (keys.ts): for one that
  // says a key of the page's does what it does. Otherwise keys typed in it
  // are its own.
  pageKeys?: boolean;
  className?: string;
  children: ReactNode;
};

// What opens it, where that isn't a trigger around it: whether it's open
// is said (open, onOpenChange), and so is the element it opens by (at).
type HeldProps = {
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  at?: RefObject<Element | null>;
};

// PopoverSurface is what opens from something, over the page: a raised
// card by what opened it, in a layer of its own, so a scrolling panel
// can't cut it off, and kept on screen. It's for the library's own parts
// (a Select's list, a Menu), which say what's in it and what opens it; a
// page uses those, or Popover. One with no trigger around it (a context
// menu, opened at the pointer) says when it's open and what it's by.
export function PopoverSurface({ placement = "bottom start", offset = 4, maxHeight, pageKeys, className, open, onOpenChange, at, children }: SurfaceProps & HeldProps) {
  return (
    <AriaPopover className={cx(css.popover, className)} placement={placement} offset={offset} maxHeight={maxHeight} isOpen={open} onOpenChange={onOpenChange} triggerRef={at} data-field={pageKeys ? undefined : ""}>
      {children}
    </AriaPopover>
  );
}

// Popover is a button and what it opens beside it: a few controls, more
// about something. Its trigger is a Button (or a ToggleButton); what's in
// it is a dialog to a screen reader, named by aria-label, and has the
// focus while it's open. Escape, or a press outside, shuts it, and the
// focus goes back to the button.
export function Popover({ trigger, "aria-label": label, children, ...surface }: Omit<SurfaceProps, "children"> & { trigger: ReactElement; "aria-label": string; children: ReactNode }) {
  return (
    <DialogTrigger>
      {trigger}
      <PopoverSurface {...surface}>
        <Dialog className={css.dialog} aria-label={label}>
          {children}
        </Dialog>
      </PopoverSurface>
    </DialogTrigger>
  );
}
