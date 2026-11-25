import { createContext, type RefObject } from "react";
import type { Place } from "./links";

// What a document's pieces are told by the Markdown that draws them: where
// the document is (its links and images lead from there), what's done to
// copy a link to a heading, and the document's own element (to find its
// headings in).
export const PlaceContext = createContext<Place | undefined>(undefined);
export const HeadingLinkContext = createContext<((anchor: string) => void) | undefined>(undefined);
export const BoxContext = createContext<RefObject<HTMLDivElement | null> | undefined>(undefined);

// toHeading scrolls to a heading by the name GitHub gives it. It says
// whether there's one.
export function toHeading(root: Element | HTMLElement, id: string): boolean {
  const h = (root as HTMLElement).querySelector(`[data-anchor="${CSS.escape(id.toLowerCase())}"]`);
  h?.scrollIntoView({ block: "start" });
  return !!h;
}
