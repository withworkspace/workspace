import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { announceNeeds, announceNotifications, getNotifications, readNotifications } from "../api";
import { useChromeStore } from "../chrome/store";
import { tellingOf, toTell } from "../notifications/telling";
import { setPref, usePrefs } from "../prefs";
import { shownAs, useAccent } from "../ui/accent";
import { toast } from "../ui/toast";

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

const supported = typeof Notification !== "undefined";

// away is whether you're not looking at this page: its tab hidden, or its
// window not the one in front. A window behind another app's is still
// "visible" to the browser while any of it shows, and what's told in it
// then (a toast in its corner) goes unseen.
export function away(): boolean {
  return document.visibilityState === "hidden" || !document.hasFocus();
}

// asked is whether this page has offered to ask the browser already:
// once a load.
let asked = false;

// useNotifications notifies of what's newly waiting while you're away from
// the page (away), with them on and the browser allowing them; it returns whether they're on and a
// switch. What's been told of is the daemon's to remember, by each need's
// key: this page only knows what's new to it.
export function useNotifications(waiting: Waiting[], open: (path: string) => void): { supported: boolean; on: boolean; toggle: () => void } {
  // On is yours, kept (prefs.ts), while the browser still allows them.
  const wanted = usePrefs((s) => s.notify);
  const [allowed, setAllowed] = useState(() => supported && Notification.permission === "granted");
  const on = wanted && allowed;
  const seen = useRef<Set<string> | null>(null);

  // The browser's answer can change outside the page (its own settings):
  // read again whenever you come back to it.
  useEffect(() => {
    if (!supported) return;
    const look = () => setAllowed(Notification.permission === "granted");
    window.addEventListener("focus", look);
    document.addEventListener("visibilitychange", look);
    return () => {
      window.removeEventListener("focus", look);
      document.removeEventListener("visibilitychange", look);
    };
  }, []);

  // Wanted, and the browser never asked: offer to ask it, once a load. Its
  // question needs a press (browsers refuse one asked unprompted).
  useEffect(() => {
    if (!supported || !wanted || asked || Notification.permission !== "default") return;
    asked = true;
    toast("ws can tell you on the desktop when something needs you, while you're elsewhere.", {
      ms: 60_000,
      action: {
        label: "Allow",
        run: () => void Notification.requestPermission().then((p) => setAllowed(p === "granted")),
      },
    });
  }, [wanted]);

  useEffect(() => {
    const keys = new Set(waiting.map((w) => w.key));
    if (seen.current && on && away()) {
      const fresh = waiting.filter((w) => !seen.current!.has(w.key));
      const tell = (which: Waiting[]) => {
        for (const w of which) {
          const n = new Notification(w.title, { body: w.body, tag: w.key, icon: markIcon(false) });
          n.onclick = () => {
            window.focus();
            open(w.path);
            n.close();
          };
        }
      };
      // The daemon says which of them no page has told of yet, so another
      // page open on the workspace, or this one reloaded, doesn't tell of
      // them again. If it can't be asked, they're told: once too often is
      // better than not at all.
      if (fresh.length) {
        announceNeeds(
          fresh.map((w) => w.key),
          [...keys],
        ).then(
          (told) => tell(fresh.filter((w) => told.includes(w.key))),
          () => tell(fresh),
        );
      }
    }
    seen.current = keys;
  }, [waiting, on, open]);

  const toggle = useCallback(() => {
    if (!supported) return;
    if (on) return setPref("notify", false);
    void Notification.requestPermission().then((p) => {
      const yes = p === "granted";
      setAllowed(yes);
      if (yes) setPref("notify", true);
    });
  }, [on]);
  // The same until it's turned on or off: what's given it isn't drawn
  // again for nothing.
  return useMemo(() => ({ supported, on, toggle }), [on, toggle]);
}

// useFeedNotifications tells of what happened (the feed's notifications)
// as it happens, for the sources and kinds you've asked to be told of
// ("Feed, and tell me": desktop in ws.local.toml): in a toast while
// you're on the page, on the desktop while you're away from it (away)
// and desktop notifications are on. latest is the newest notification's number, as
// the workspace's view carries it: when it moves, what's new since is
// read, and the daemon says which of it no page has told of yet, as it
// does for needs, so it's told once, toast or desktop, however many pages
// are open.
export function useFeedNotifications(latest: number | undefined, on: boolean, open: (path: string) => void) {
  const seen = useRef<number | undefined>(undefined);
  useEffect(() => {
    if (latest === undefined) return;
    const was = seen.current;
    seen.current = latest;
    // The first it hears of is where it starts: nothing's new to a page
    // that's just opened.
    if (was === undefined || latest <= was) return;
    const hidden = away();
    if (hidden && !on) return;
    void getNotifications()
      .then(async (feed) => {
        const fresh = toTell(feed.notifications, was, hidden, window.location.pathname);
        if (!fresh.length) return;
        const claimed = await announceNotifications(fresh.map((n) => n.n));
        const told = fresh.filter((f) => claimed.includes(f.n));
        if (!hidden) {
          const t = tellingOf(told);
          if (!t) return;
          toast(t.text, {
            ms: 8000,
            action: {
              label: t.label,
              run: () => {
                if (t.n !== undefined && t.to) {
                  void readNotifications([t.n]).catch(() => {});
                  open(t.to);
                } else useChromeStore.getState().setBell(true);
              },
            },
          });
          return;
        }
        for (const n of told) {
          const note = new Notification(n.text, { tag: n.ref, icon: markIcon(false) });
          note.onclick = () => {
            window.focus();
            open(n.to || "/notifications");
            note.close();
          };
        }
      })
      .catch(() => {
        // It's in the feed either way: this only tells of it as well.
      });
  }, [latest, on, open]);
}
