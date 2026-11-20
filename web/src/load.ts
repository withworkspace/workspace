import { type DependencyList, type Dispatch, type SetStateAction, useCallback, useEffect, useReducer, useRef, useState } from "react";

// Loaded is what a load gave (value), or why it didn't (error), a way to
// change value in place, as a write that answered with it does (which
// clears the error), and a way to load it again, as after a write that
// didn't.
export interface Loaded<T> {
  value: T | undefined;
  error?: string;
  set: Dispatch<SetStateAction<T | undefined>>;
  reload: Reload;
}

// Reload loads it again, keeping what's shown until the answer comes. Its
// promise settles once that answer's shown (or the load's failed, or
// there's no longer anything to show it): what did the write waits on it
// before it's no longer busy, so a button isn't back, beside what it's
// just changed, before the change is.
export type Reload = () => Promise<void>;

// useLoaded is what load gives, loaded again whenever deps change, and on
// reload; an answer that comes after a later load has begun is dropped.
// While it's loading again for new deps, value is what came before (keep),
// or nothing. A null load is nothing to load: value is cleared.
export function useLoaded<T>(load: (() => Promise<T>) | null, deps: DependencyList, opts: { keep?: boolean } = {}): Loaded<T> {
  const [value, set] = useState<T>();
  const [error, setError] = useState<string>();
  const [again, reload] = useReducer((n: number) => n + 1, 0);
  const reloading = useRef(false);
  // Those waiting on a reload: told by the first load begun after they
  // asked that shows something. One that goes stale first hands them on.
  const waiting = useRef<(() => void)[]>([]);
  const gone = useRef(false);
  useEffect(() => {
    const keep = opts.keep || reloading.current;
    reloading.current = false;
    const mine = waiting.current.splice(0);
    const shown = () => mine.splice(0).forEach((tell) => tell());
    if (!load) {
      set(undefined);
      setError(undefined);
      shown();
      return;
    }
    let stale = false;
    if (!keep) {
      set(undefined);
      setError(undefined);
    }
    load().then(
      (v) => {
        if (stale) return;
        set(v);
        setError(undefined);
        shown();
      },
      (e: Error) => {
        if (stale) return;
        setError(e?.message ?? String(e));
        shown();
      },
    );
    return () => {
      stale = true;
      if (gone.current) shown();
      else waiting.current.unshift(...mine.splice(0));
    };
    // load is a new closure each draw; deps say when it's a new load.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, again]);
  // Gone, there's nothing left to wait for. (Set on each mount: in
  // development React mounts twice.)
  useEffect(() => {
    gone.current = false;
    return () => {
      gone.current = true;
      waiting.current.splice(0).forEach((tell) => tell());
    };
  }, []);
  const loadAgain = useCallback(() => {
    reloading.current = true;
    const done = new Promise<void>((tell) => waiting.current.push(tell));
    reload();
    return done;
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

// bursts tells of a change at once, and of those that follow within a
// moment of it (ms) once, after it: a write that moves several of a
// mission's records is told of each (approving a bundle, eight within a
// few milliseconds), and each would otherwise be a read of the whole
// page. stop tells of nothing more.
export function bursts(tell: () => void, ms = 80): { changed: () => void; stop: () => void } {
  let quiet: ReturnType<typeof setTimeout> | undefined;
  let more = false;
  const changed = () => {
    if (quiet !== undefined) {
      more = true;
      return;
    }
    tell();
    quiet = setTimeout(() => {
      quiet = undefined;
      if (!more) return;
      more = false;
      changed();
    }, ms);
  };
  return {
    changed,
    stop: () => {
      clearTimeout(quiet);
      quiet = undefined;
      more = false;
    },
  };
}
