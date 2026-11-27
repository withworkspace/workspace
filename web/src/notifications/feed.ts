import type { Notification } from "../api";
import type { IconName } from "../ui/Icon";
import { dayKey } from "../ui/when";

// The feed, as its page draws it: a notification's icon by where it's
// from, and the feed by day.

// sourceIcon is the icon of where a notification's from: one that went
// wrong has the alert, wherever it's from.
export function sourceIcon(n: Pick<Notification, "source" | "attention">): IconName {
  if (n.attention) return "alert";
  switch (n.source) {
    case "missions":
      return "flag";
    case "pulls":
      return "pr";
    case "plugins":
      return "box";
    case "ws":
      return "info";
    case "automations":
      return "bolt";
  }
  // What a plugin told of (plugin:jira): a plugin's, as its updates are.
  if (n.source.startsWith("plugin:")) return "box";
  return "bell";
}

export interface Day {
  // key is the day, as dayKey says it; the notifications are as the feed
  // has them, newest first.
  key: string;
  notifications: Notification[];
}

// byDay is the feed in days, the latest first: each day's notifications
// in the order the feed has them.
export function byDay(feed: Notification[]): Day[] {
  const days: Day[] = [];
  for (const n of feed) {
    const key = dayKey(n.at);
    const last = days[days.length - 1];
    if (last?.key === key) last.notifications.push(n);
    else days.push({ key, notifications: [n] });
  }
  return days;
}

// unreadOf is the numbers of those not read.
export const unreadOf = (feed: Notification[]): number[] => feed.filter((n) => !n.read).map((n) => n.n);
