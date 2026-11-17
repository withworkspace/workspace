import { useEffect, useRef, useState } from "react";
import { shownAs, useAccent } from "../ui/accent";

// What needs you, told outside the page: a dot on the tab's icon whenever
// something does, and (if you turn them on) a desktop notification for
// each new thing while the tab is in the background.

export interface Waiting {
  key: string; // changes when what's waiting changes
  title: string;
  body: string;
  path: string;
}

// markIcon is ws's mark, as the tab's icon and a notification's: the tile
// in the accent's run as the page draws it (styles.css's --brand-1..3, the
// same in either theme), its stroke white, and with an amber dot when
// something needs you. It's built from the page because an icon is drawn
// outside it, where its styles don't reach; the white and the amber are the
// icon's own, whatever the accent.
export function markIcon(dot: boolean): string {
  const [a, b, c] = ["--brand-1", "--brand-2", "--brand-3"].map(shownAs);
  return (
    "data:image/svg+xml," +
    encodeURIComponent(
      `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="${a}"/><stop offset="0.62" stop-color="${b}"/><stop offset="1" stop-color="${c}"/></linearGradient></defs><rect width="32" height="32" rx="8" fill="url(#g)"/><path d="M8 11.5 11.5 21 16 14l4.5 7L24 11.5" fill="none" stroke="#fff" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"/>` +
        (dot ? `<circle cx="26" cy="6" r="6" fill="#f0a020" stroke="#fff" stroke-width="2"/>` : "") +
        `</svg>`,
    )
  );
}

// useFaviconBadge keeps the tab's icon the mark, in the accent as it is
// now, with the dot while something needs you.
export function useFaviconBadge(on: boolean) {
  const accent = useAccent();
  useEffect(() => {
    let link = document.querySelector<HTMLLinkElement>("link[rel=icon]");
    if (!link) {
      link = document.createElement("link");
      link.rel = "icon";
      document.head.appendChild(link);
    }
    link.type = "image/svg+xml";
    link.href = markIcon(on);
  }, [on, accent]);
}

const stored = () => {
  try {
    return localStorage.getItem("ws-notify") === "on";
  } catch {
    return false;
  }
};

const supported = typeof Notification !== "undefined";

// useNotifications notifies of what's newly waiting while the page is
// hidden, once you've turned them on; it returns whether they're on and a
// switch.
export function useNotifications(waiting: Waiting[], open: (path: string) => void): { supported: boolean; on: boolean; toggle: () => void } {
  const [on, setOn] = useState(() => supported && stored() && Notification.permission === "granted");
  const seen = useRef<Set<string> | null>(null);

  useEffect(() => {
    const keys = new Set(waiting.map((w) => w.key));
    if (seen.current && on && document.visibilityState === "hidden") {
      for (const w of waiting) {
        if (seen.current.has(w.key)) continue;
        const n = new Notification(w.title, { body: w.body, tag: w.key, icon: markIcon(false) });
        n.onclick = () => {
          window.focus();
          open(w.path);
          n.close();
        };
      }
    }
    seen.current = keys;
  }, [waiting, on, open]);

  const toggle = () => {
    if (!supported) return;
    if (on) {
      setOn(false);
      try {
        localStorage.removeItem("ws-notify");
      } catch {
        // nothing kept
      }
      return;
    }
    Notification.requestPermission().then((p) => {
      const yes = p === "granted";
      setOn(yes);
      try {
        if (yes) localStorage.setItem("ws-notify", "on");
      } catch {
        // nothing kept
      }
    });
  };
  return { supported, on, toggle };
}
