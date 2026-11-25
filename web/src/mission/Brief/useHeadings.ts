import { type RefObject, useEffect, useState } from "react";
import type { DocId } from "./docState";
import { docOf, docsIn, headingsIn } from "./find";

// A heading of the brief or the plan, as it's drawn.
export interface Heading {
  doc: DocId;
  text: string;
  el: HTMLElement;
}

// useHeadings is the documents' headings, as they're drawn: read again as
// they change. docs is the documents' element: its headings are read from
// it, so the list is theirs whether they're being read as changes or
// edited.
export function useHeadings(docs: RefObject<HTMLElement | null>): Heading[] {
  const [heads, setHeads] = useState<Heading[]>([]);
  // (An effect, not a layout effect: the documents' element comes after
  // the outline's, and its ref isn't set until the layout's done.)
  useEffect(() => {
    const el = docs.current;
    if (!el) return;
    let timer = 0;
    const look = () => {
      const found: Heading[] = [];
      for (const doc of docsIn(el)) {
        for (const h of headingsIn(doc)) found.push({ doc: docOf(doc), text: h.textContent?.trim() ?? "", el: h });
      }
      setHeads((was) => (was.length === found.length && was.every((h, i) => h.el === found[i].el && h.text === found[i].text) ? was : found));
    };
    look();
    // (The documents draw themselves after this: looked at again once they have.)
    const first = requestAnimationFrame(look);
    const seen = new MutationObserver(() => {
      clearTimeout(timer);
      timer = window.setTimeout(look, 40);
    });
    seen.observe(el, { childList: true, subtree: true, characterData: true });
    return () => {
      seen.disconnect();
      clearTimeout(timer);
      cancelAnimationFrame(first);
    };
  }, [docs]);
  return heads;
}
