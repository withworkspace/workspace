import { useEffect, useState } from "react";
import { type DocHit, docAddress, searchDocuments } from "../api";
import { humanError } from "../ui/text";

// Search over the spaces' documents: ranked, each led to the section
// that says it best; kept to a kind, and where one stands, once a kind's
// chosen. Only the default branch is searched: a document in flight is
// read as it is there, in its space.

export const hitTo = (h: DocHit) => docAddress(`${h.repo}:${h.path}`) + (h.anchor ? `#${h.anchor}` : "");

// useDocSearch is what's found for q, kept to kind and state, as it's
// typed (a moment after the last key).
export function useDocSearch(q: string, kind: string, state: string) {
  const [hits, setHits] = useState<DocHit[]>();
  const [more, setMore] = useState(0);
  const [error, setError] = useState<string>();
  useEffect(() => {
    if (!q.trim()) {
      setHits(undefined);
      return;
    }
    let stale = false;
    const t = setTimeout(() => {
      searchDocuments(q, { kind, state, limit: 30 }).then(
        (r) => {
          if (stale) return;
          setHits(r.hits);
          setMore(r.more);
          setError(undefined);
        },
        (e: Error) => !stale && setError(humanError(e.message)),
      );
    }, 200);
    return () => {
      stale = true;
      clearTimeout(t);
    };
  }, [q, kind, state]);
  return { hits, more, error };
}
