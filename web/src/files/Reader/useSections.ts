import { type RefObject, useEffect, useState } from "react";
import type { Section } from "../reading";

// useSections is a document's sections and its title (its first h1), read
// from its text as it's drawn: the h2 and h3 headings the markdown named.
// Read again whenever what's drawn changes, as it's drawn after it loads
// (text names that: it's read again when the document is).
export function useSections(article: RefObject<HTMLElement | null>, text: string | undefined): { title?: string; sections: Section[] } {
  const [found, setFound] = useState<{ title?: string; sections: Section[] }>({ sections: [] });
  useEffect(() => {
    const el = article.current;
    if (!el) return setFound({ sections: [] });
    const read = () => {
      const title = el.querySelector("h1")?.textContent?.trim() || undefined;
      const sections = [...el.querySelectorAll<HTMLElement>("h2[data-anchor], h3[data-anchor]")].map(
        (h): Section => ({ anchor: h.dataset.anchor ?? "", level: h.tagName === "H2" ? 2 : 3, text: h.textContent?.trim() ?? "" }),
      );
      setFound((was) => (was.title === title && sameSections(was.sections, sections) ? was : { title, sections }));
    };
    read();
    const watch = new MutationObserver(read);
    watch.observe(el, { childList: true, subtree: true, characterData: true });
    return () => watch.disconnect();
  }, [article, text]);
  return found;
}

const sameSections = (a: Section[], b: Section[]) => a.length === b.length && a.every((s, i) => s.anchor === b[i].anchor && s.level === b[i].level && s.text === b[i].text);
