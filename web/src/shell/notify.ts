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
// eclipse (brand:svg/mark.svg), with no tile. The light behind is the
// wordmark's run, a disc in front of it is the ink, and the icon follows the
// browser's theme as brand's does, so it suits the tab bar it sits in: on a
// light one the run is --brand-1..3 and the disc is the ink at 80%, on a dark
// one the run is --brand-on-dark-1..4 and the disc is that run's first step,
// dimmed, so the light behind is always the brighter. While something needs
// you it is lit: the light behind turns amber (brand:svg/mark-lit.svg). It's
// built from the page because an icon is drawn outside it, where its styles
// don't reach; the ink and the amber are the icon's own, whatever the accent.
export function markIcon(lit: boolean): string {
  const light = ["--brand-1", "--brand-2", "--brand-3"].map(shownAs);
  const dark = ["--brand-on-dark-1", "--brand-on-dark-2", "--brand-on-dark-3", "--brand-on-dark-4"].map(shownAs);
  const run = (id: string, colours: string[], offsets: number[]) =>
    `<linearGradient id="${id}" x1="${RUN[0]}" y1="0" x2="${RUN[1]}" y2="0" gradientUnits="userSpaceOnUse">` +
    colours.map((c, i) => `<stop offset="${offsets[i]}" stop-color="${c}"/>`).join("") +
    `</linearGradient>`;
  const style =
    `.g{fill:url(#l)}.i{fill:${INK};fill-opacity:0.8}` +
    `@media (prefers-color-scheme:dark){.g{fill:url(#d)}.i{fill:${dark[0]};fill-opacity:0.45}}`;
  return (
    "data:image/svg+xml," +
    encodeURIComponent(
      `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512"><defs><style>${style}</style>` +
        run("l", light, [0, 0.62, 1]) +
        run("d", dark, [0, 0.55, 0.78, 1]) +
        `</defs>` +
        (lit ? `<path d="${BEHIND}" fill="${NEEDS}"/>` : `<path d="${BEHIND}" class="g"/>`) +
        `<path d="${FRONT}" class="i"/></svg>`,
    )
  );
}

// The eclipse as brand draws it, on its 512 square (brand:svg/mark.svg): what
// shows of the disc behind, the disc in front, and where the run starts and
// ends across them.
const BEHIND = "M155.56 86.23A192 192 0 1 1 425.77 356.44A224 224 0 0 0 155.56 86.23Z";
const FRONT = "M16 304a192 192 0 1 0 384 0a192 192 0 1 0 -384 0Z";
const RUN = [112, 496];
const INK = "#0d0b1e";
const NEEDS = "#f0a020";

// useFaviconBadge keeps the tab's icon the mark, in the accent as it is
// now, lit while something needs you.
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
