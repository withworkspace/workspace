import { useCallback, useState } from "react";
import { type ChromeState, read, write } from "./chromeState";

const key = (root: string) => `ws:chrome:${root}`;
// Which workspace this browser last arranged here: its arrangement is
// what the page opens with, before the workspace has said which it is.
const lastKey = "ws:chrome:last";

function load(root: string): ChromeState {
  try {
    const of = root || localStorage.getItem(lastKey) || "";
    if (root) localStorage.setItem(lastKey, root);
    return read(of ? localStorage.getItem(key(of)) : undefined);
  } catch {
    return read(undefined);
  }
}

// useChrome is how this window is arranged, kept in the browser for the
// workspace: what it was when the page last closed, and each change as
// it's made. A change is a function of the arrangement (chromeState.ts's:
// hideSidebar). Until the workspace is known (no root yet)
// it's the arrangement of the one last open here, which is nearly always
// the same one, so a hidden sidebar doesn't show for a moment on the way
// in.
export function useChrome(
  root: string | undefined,
): [ChromeState, (change: (s: ChromeState) => ChromeState) => void] {
  const of = root ?? "";
  const [state, setState] = useState(() => load(of));
  const [was, setWas] = useState(of);
  if (was !== of) {
    setWas(of);
    setState(load(of));
  }
  const change = useCallback(
    (by: (s: ChromeState) => ChromeState) =>
      setState((s) => {
        const next = by(s);
        try {
          if (of) localStorage.setItem(key(of), write(next));
        } catch {
          // Nowhere to keep it: it holds while the page is open.
        }
        return next;
      }),
    [of],
  );
  return [state, change];
}
