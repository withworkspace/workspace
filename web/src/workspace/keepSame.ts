// keepSame is now, with every part of it that's the same as it was being
// the one it was: the very object, not one like it. The workspace is read
// whole each time it changes, and what's read is all new objects, though
// most of it says what it said before. Kept the same, a part that hasn't
// changed is the same to whatever reads it (a store's selector, a memo's
// deps), which is then not drawn or worked out again; and a read that
// changed nothing is the workspace it was.
//
// Lists are matched by place: one with something put in at its start is
// new from there on. That's a part drawn again that needn't have been,
// never one that's stale.
export function keepSame<T>(was: unknown, now: T): T {
  if (Object.is(was, now)) return now;
  if (Array.isArray(was) && Array.isArray(now)) {
    let same = was.length === now.length;
    const kept = now.map((v, i) => {
      const k = keepSame(was[i], v);
      if (k !== was[i]) same = false;
      return k;
    });
    return (same ? was : kept) as T;
  }
  if (plain(was) && plain(now)) {
    const keys = Object.keys(now);
    let same = Object.keys(was).length === keys.length;
    const kept: Record<string, unknown> = {};
    for (const k of keys) {
      kept[k] = keepSame(was[k], now[k]);
      if (!(k in was) || kept[k] !== was[k]) same = false;
    }
    return (same ? was : kept) as T;
  }
  return now;
}

// plain is something read from JSON as an object: not a list, nor null.
const plain = (v: unknown): v is Record<string, unknown> => typeof v === "object" && v !== null && !Array.isArray(v);
