import type { Notification } from "../api";

// What's told of as it happens: the feed's new notifications whose source
// or kind you've asked to be told of ("Feed, and tell me": desktop in the
// file). With the page in front it's said in a toast; with it hidden, on
// the desktop, if desktop notifications are on. Either way the daemon says
// which no page has told of yet, so it's told once, wherever you are.

// toTell is what's new since was in the feed and to be told of: not read,
// and with the page in front, not about the page you're on, which you're
// looking at already.
export function toTell(feed: Notification[], was: number, hidden: boolean, here: string): Notification[] {
  return feed.filter((n) => n.n > was && n.desktop && !n.read && (hidden || !n.to || n.to !== here));
}

// A Telling is what a toast says of what's being told of, and what its
// button does: Open goes to the one notification's page (reading it), Show
// opens the bell's list.
export interface Telling {
  text: string;
  label: "Open" | "Show";
  // n is the notification Open reads, and to where it goes.
  n?: number;
  to?: string;
}

// tellingOf is the toast for what's told: the one, by what it says, or
// several, by how many.
export function tellingOf(told: Notification[]): Telling | undefined {
  if (told.length === 0) return undefined;
  if (told.length === 1) {
    const [n] = told;
    return n.to ? { text: n.text, label: "Open", n: n.n, to: n.to } : { text: n.text, label: "Show" };
  }
  return { text: `${told.length} new notifications`, label: "Show" };
}
