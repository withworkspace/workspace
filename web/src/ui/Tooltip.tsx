import type { ReactElement, ReactNode } from "react";
import { Tooltip as AriaTooltip, type TooltipProps as AriaTooltipProps, TooltipTrigger } from "react-aria-components";
import css from "./Tooltip.module.css";

// Tooltip says more about something that's pressed, when the pointer
// rests on it or the keys' focus comes to it: what it does, its key, why
// it can't be used. It's react-aria's tooltip: over the page by what it's
// about, kept on screen, gone on Escape or when the pointer leaves, and
// read to a screen reader as the thing's description.
//
// What it's about is its child, which has to be something that takes the
// focus: one of the library's buttons. Those take a title and draw this
// themselves, so a page seldom writes it. Words alone that have more to
// say (a cut-off path, a time) keep the browser's title.
export function Tooltip({ text, placement = "top", children }: { text: ReactNode; placement?: AriaTooltipProps["placement"]; children: ReactElement }) {
  return (
    <TooltipTrigger delay={500} closeDelay={0}>
      {children}
      <AriaTooltip className={css.tooltip} placement={placement} offset={6}>
        {text}
      </AriaTooltip>
    </TooltipTrigger>
  );
}
