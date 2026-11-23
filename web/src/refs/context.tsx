import { createContext, type ReactNode } from "react";

// Refs are the names a page knows (c1, d2, t3, a capsule, a document) and
// how to draw each: text that mentions one shows it with its hover card.
export interface Refs {
  pattern: RegExp; // matches any of them, as whole words
  render: (ref: string, key: number) => ReactNode | null;
}

export const RefContext = createContext<Refs | undefined>(undefined);

// linkRefs draws the refs in text.
export function linkRefs(text: string, refs: Refs | undefined, at = 0): ReactNode[] {
  if (!refs) return [text];
  const out: ReactNode[] = [];
  let last = 0;
  for (const m of text.matchAll(refs.pattern)) {
    const i = m.index ?? 0;
    const drawn = refs.render(m[0], at + i);
    if (!drawn) continue;
    if (i > last) out.push(text.slice(last, i));
    out.push(drawn);
    last = i + m[0].length;
  }
  if (last < text.length) out.push(text.slice(last));
  return out;
}
