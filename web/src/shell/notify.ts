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

// markIcon is ws's mark, as the tab's icon and a notification's: brand's
// square-w (brand:svg/square-w.svg), a tile with a 12% corner radius and a
// solid white "w" in its bottom-left, drawn in the accent's run as the page
// draws it (styles.css's --brand-1..3, the same in either theme) on brand's
// diagonal, with an amber dot top-right, clear of the "w", when something
// needs you. It's built from the page because an icon is drawn outside it,
// where its styles don't reach; the white and the amber are the icon's own,
// whatever the accent.
export function markIcon(dot: boolean): string {
  const [a, b, c] = ["--brand-1", "--brand-2", "--brand-3"].map(shownAs);
  return (
    "data:image/svg+xml," +
    encodeURIComponent(
      `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${ICON} ${ICON}"><defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="${a}"/><stop offset="0.62" stop-color="${b}"/><stop offset="1" stop-color="${c}"/></linearGradient></defs><rect width="${ICON}" height="${ICON}" rx="${scaled(BRAND_RX)}" fill="url(#g)"/><path d="${W}" fill="#fff"/>` +
        (dot ? `<circle cx="26" cy="6" r="6" fill="#f0a020" stroke="#fff" stroke-width="2"/>` : "") +
        `</svg>`,
    )
  );
}

// square-w's geometry as brand draws it, on its 512 tile: the corner radius
// and the "w" outline (padded 14% in from the bottom-left), copied from
// brand:svg/square-w.svg. markIcon draws them scaled to its own tile.
const BRAND_TILE = 512;
const BRAND_RX = 61;
const BRAND_W =
  "M98.34465 440.32 71.68 283.98072249999996H104.36093625000001L124.10530125000001 418.33693125H129.15264375L156.245675 283.98072249999996H209.25893000000002L236.35196125 418.33693125H241.39930375L261.14366875 283.98072249999996H293.824605L267.159955 440.32H212.36900500000002L185.27597375 305.96379125H180.22863125L153.1356 440.32Z";
const ICON = 32;

// scaled is a length on brand's tile on the icon's, to a hundredth.
const scaled = (n: number) => String(Math.round((n * ICON * 100) / BRAND_TILE) / 100);

// The "w" is all straight lines (M, L, H, Z), so scaling it is scaling
// every number in it.
const W = BRAND_W.replace(/\d+(\.\d+)?/g, (n) => scaled(Number(n)));

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
