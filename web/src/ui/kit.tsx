import { useEffect, useLayoutEffect, useState } from "react";

// Hooks every page uses.

// useTitle names the browser tab after the page.
let workspaceName = "ws";
let waitingCount = 0;
let pageTitle = "";

function applyTitle() {
  const parts = [pageTitle, workspaceName].filter(Boolean);
  document.title = (waitingCount ? `(${waitingCount}) ` : "") + parts.join(" · ");
}

export function setTitleContext(name: string, waiting: number) {
  workspaceName = name || "ws";
  waitingCount = waiting;
  applyTitle();
}

export function useTitle(title: string | undefined) {
  useEffect(() => {
    pageTitle = title ?? "";
    applyTitle();
  }, [title]);
}

// fitText sizes a textarea to what's in it, up to max pixels. One that
// isn't laid out (in a view that's hidden) has no height to go by: it's
// left at its rows, and fitted when it's shown.
function fitText(el: HTMLTextAreaElement, max: number) {
  if (el.getClientRects().length === 0) {
    el.style.height = "";
    return;
  }
  el.style.height = "auto";
  el.style.height = `${Math.min(el.scrollHeight + 2, max)}px`;
  el.style.overflowY = el.scrollHeight + 2 > max ? "auto" : "hidden";
}

// useAutosize grows a textarea with what's typed, up to max pixels, and
// fits it again when its width changes: when a hidden view it's in is
// shown, or the window's resized and its lines wrap differently.
export function useAutosize(ref: React.RefObject<HTMLTextAreaElement | null>, value: string, max = 320) {
  useLayoutEffect(() => {
    const el = ref.current;
    if (el) fitText(el, max);
  }, [ref, value, max]);
  useEffect(() => {
    const el = ref.current;
    if (!el || typeof ResizeObserver === "undefined") return;
    // Only its width: fitting it changes its height, which would come
    // back here.
    let width = el.clientWidth;
    const watch = new ResizeObserver(() => {
      if (el.clientWidth === width) return;
      width = el.clientWidth;
      fitText(el, max);
    });
    watch.observe(el);
    return () => watch.disconnect();
  }, [ref, max]);
}

// useMedia is whether a media query matches now, kept up to date.
export function useMedia(query: string): boolean {
  const [on, setOn] = useState(() => window.matchMedia(query).matches);
  useEffect(() => {
    const m = window.matchMedia(query);
    const changed = () => setOn(m.matches);
    m.addEventListener("change", changed);
    changed();
    return () => m.removeEventListener("change", changed);
  }, [query]);
  return on;
}

// useDraft keeps what's typed in a composer across visits, per key, in
// this browser.
export function useDraft(key: string): [string, (v: string) => void] {
  const storeKey = `ws-draft:${key}`;
  const load = () => {
    try {
      return localStorage.getItem(storeKey) ?? "";
    } catch {
      return "";
    }
  };
  const [state, setState] = useState(() => ({ key: storeKey, value: load() }));
  // Another conversation in the same composer: its own draft.
  const value = state.key === storeKey ? state.value : load();
  if (state.key !== storeKey) setState({ key: storeKey, value });
  const set = (v: string) => {
    setState({ key: storeKey, value: v });
    try {
      if (v) localStorage.setItem(storeKey, v);
      else localStorage.removeItem(storeKey);
    } catch {
      // storage off: the draft lives as long as the page
    }
  };
  return [value, set];
}
