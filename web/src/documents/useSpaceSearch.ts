import { useEffect, useState } from "react";
import { type DocHit, searchDocuments } from "../api";
import { humanError } from "../ui/text";

// useSpaceSearch is what a search of one space finds, by document (each
// led to the section that says it best), as it's typed (a moment after
// the last key).
export function useSpaceSearch(repo: string, q: string) {
  const [hits, setHits] = useState<Map<string, DocHit>>();
  const [error, setError] = useState<string>();
  useEffect(() => {
    if (!q.trim()) {
      setHits(undefined);
      setError(undefined);
      return;
    }
    let stale = false;
    const t = setTimeout(() => {
      searchDocuments(q, { repos: [repo], limit: 500 }).then(
        (r) => {
          if (stale) return;
          setHits(new Map(r.hits.filter((h) => h.repo === repo).map((h) => [h.path, h])));
          setError(undefined);
        },
        (e: Error) => !stale && setError(humanError(e.message)),
      );
    }, 200);
    return () => {
      stale = true;
      clearTimeout(t);
    };
  }, [repo, q]);
  return { hits, error };
}
