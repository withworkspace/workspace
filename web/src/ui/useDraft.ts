import { useState } from "react";

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
