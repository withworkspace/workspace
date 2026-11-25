import { type RefObject, useMemo } from "react";
import type { Space } from "../api";
import { DocumentsSearch } from "./DocumentsSearch";

// SearchBox is a search of every space's documents, drawn as a
// DocumentsSearch: the kinds it can be kept to are the spaces', each
// once, and where one of them can stand.
export function SearchBox({ q, setQ, inputRef, spaces, kind, setKind, state, setState, onKey }: { q: string; setQ: (q: string) => void; inputRef: RefObject<HTMLInputElement | null>; spaces: Space[]; kind: string; setKind: (k: string) => void; state: string; setState: (s: string) => void; onKey: (e: React.KeyboardEvent) => void }) {
  const kinds = useMemo(() => {
    const out = new Map<string, { title: string; states?: string[] }>();
    for (const s of spaces) for (const k of s.kinds) out.set(k.name, { title: k.title, states: k.states });
    return out;
  }, [spaces]);
  return (
    <DocumentsSearch
      inputRef={inputRef}
      q={q}
      setQ={setQ}
      onKey={onKey}
      kinds={[...kinds].map(([name, k]) => ({ name, title: k.title }))}
      kind={kind}
      setKind={setKind}
      states={kinds.get(kind)?.states ?? []}
      state={state}
      setState={setState}
    />
  );
}
