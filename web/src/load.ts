import { type DependencyList, type Dispatch, type SetStateAction, useCallback, useEffect, useReducer, useRef, useState } from "react";

// Loaded is what a load gave (value), or why it didn't (error), a way to
// change value in place, as a write that answered with it does (which
// clears the error), and a way to load it again, as after a write that
// didn't.
export interface Loaded<T> {
  value: T | undefined;
  error?: string;
  set: Dispatch<SetStateAction<T | undefined>>;
  // reload loads it again, keeping what's shown until the answer comes.
  reload: () => void;
}

// Soon is how a load shows what it has so far, before its answer: what's
// on disk, say, before what GitHub says. It's dropped once the load's
// stale, as the answer is.
export type Soon<T> = Dispatch<SetStateAction<T | undefined>>;

// useLoaded is what load gives, loaded again whenever deps change, and on
// reload; an answer that comes after a later load has begun is dropped.
// While it's loading again for new deps, value is what came before (keep),
// or nothing. A null load is nothing to load: value is cleared.
export function useLoaded<T>(load: ((soon: Soon<T>) => Promise<T>) | null, deps: DependencyList, opts: { keep?: boolean } = {}): Loaded<T> {
  const [value, set] = useState<T>();
  const [error, setError] = useState<string>();
  const [again, reload] = useReducer((n: number) => n + 1, 0);
  const reloading = useRef(false);
  useEffect(() => {
    const keep = opts.keep || reloading.current;
    reloading.current = false;
    if (!load) {
      set(undefined);
      setError(undefined);
      return;
    }
    let stale = false;
    if (!keep) {
      set(undefined);
      setError(undefined);
    }
    const soon: Soon<T> = (v) => {
      if (stale) return;
      set(v);
      setError(undefined);
    };
    load(soon).then(
      (v) => {
        if (stale) return;
        set(v);
        setError(undefined);
      },
      (e: Error) => !stale && setError(e?.message ?? String(e)),
    );
    return () => {
      stale = true;
    };
    // load is a new closure each draw; deps say when it's a new load.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, again]);
  const loadAgain = useCallback(() => {
    reloading.current = true;
    reload();
  }, []);
  const setValue: Dispatch<SetStateAction<T | undefined>> = useCallback((v) => {
    set(v);
    setError(undefined);
  }, []);
  return { value, error, set: setValue, reload: loadAgain };
}

// useLatest numbers a component's loads, for one that isn't a useLoaded
// (it sets several things, or keeps what it's given): begin() at the start
// of each load gives a fresh() that's true while it's the latest begun and
// the component's still there. Check it before setting anything.
export function useLatest(): () => () => boolean {
  const n = useRef(0);
  useEffect(
    () => () => {
      n.current++;
    },
    [],
  );
  return useCallback(() => {
    const mine = ++n.current;
    return () => mine === n.current;
  }, []);
}
