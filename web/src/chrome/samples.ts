import type { Workspace } from "../api";
import sorrel from "../mission/fixtures/sorrel/workspace.json";
import { arrange, type Place, placesOf } from "../shell/places";
import type { IconName } from "../ui/Icon";
import type { StatusInfo } from "../ui/Status";
import { type Need, waitingOf } from "./need";

// What the chrome's stories draw: Sorrel, the workspace the stories'
// missions were taken from. Its needs and places are worked out from it
// as the app works them out; only what ws doesn't keep yet is written by
// hand, and says so.

export const workspace = sorrel as unknown as Workspace;

// What needs you there. The daemon gives the list now (workspace.needs),
// but what was taken predates that, so until it's taken again
// (scripts/story-fixtures.mjs) it's worked out here by the daemon's rule:
// the missions in your court, then the conversations that are your turn.
export const needs: Need[] = workspace.needs ?? [
  ...workspace.missions
    .filter((m) => m.headline.group === "needs-you")
    .map((m): Need => ({
      key: `mission|${m.slug}|${m.headline.kind}`,
      of: "mission",
      ref: m.slug,
      kind: m.headline.kind,
      label: m.headline.label,
      title: m.title,
      text: m.headline.text || m.headline.label,
      to: `/missions/${m.slug}`,
    })),
  ...workspace.conversations
    .filter((t) => !!t.waiting && t.status === "open")
    .map((t): Need => ({
      key: `conversation||${t.id}|${t.updated}`,
      of: "conversation",
      ref: t.id,
      kind: "your-turn",
      title: t.title || t.opening || "Conversation",
      text: "Agent replied: your turn.",
      to: `/conversations/${t.id}`,
    })),
];

// placesFor is the sidebar's places with their counts, for some needs.
export function placesFor(now: Need[]): Place[] {
  return arrange(placesOf(workspace, waitingOf(now))).sidebar;
}

// A crew asking something: a need that arrives while you're looking.
// Written here: it's the mission c1 is at work on in what was taken.
export const crewAsks: Need = {
  key: "mission|dark-mode-for-the-dashboard|crew-asks|c1|1",
  of: "mission",
  ref: "dark-mode-for-the-dashboard",
  kind: "crew-asks",
  label: "? asks",
  title: "Dark mode for the dashboard",
  text: "c1 asks: Do charts follow the theme too, or keep their own palette?",
  to: "/missions/dark-mode-for-the-dashboard",
};

// Pull requests asking for your review by name, as the daemon lists them
// after what's yours. Written here: what was taken has none. The last is
// a stack, one need at its lowest layer that asks.
const pull = (repo: string, number: number, title: string, text: string, key = `${repo}#${number}`): Need => ({
  key: `pull|${key}|`,
  of: "pull",
  ref: `${repo}#${number}`,
  kind: "your-review",
  title,
  text,
  to: `/reviews/${repo}/${number}`,
});
export const toReview: Need[] = [
  pull("api", 212, "Rate limit by key, not by address", "api#212, from ines, needs your review."),
  pull("web", 87, "Invoice table keeps its header on scroll", "web#87, from tomas, needs your review again."),
  pull("api", 205, "Webhook signing", "api#205, from ines, needs your review, plus 2 more in the stack.", "api/stack-4"),
];

// More of them than the list shows: the rest are a press away, in Reviews.
export const manyToReview: Need[] = [
  ...toReview,
  pull("api", 214, "Drop the v1 export endpoint", "api#214, from priya, needs your review."),
  pull("docs", 31, "Say how retries back off", "docs#31, from tomas, needs your review."),
  pull("web", 90, "Totals to the penny in the summary", "web#90, from priya, needs your review."),
];

// What's new at some places since you last looked. Written here: ws
// keeps no marks yet.
export const news: Record<string, string> = {
  reviews: "ines asks your team for a review",
  documents: "ines amended ADR 0003",
  files: "api's main moved on",
};

// Pins, for Focus. Written here: nobody has pinned anything in what was
// taken. A pinned mission shows how it stands; other things their kind.
export interface SamplePin {
  name: string;
  to: string;
  title: string;
  status?: StatusInfo;
  icon?: IconName;
}

export const pins: SamplePin[] = [
  {
    name: "dark-mode-for-the-dashboard",
    to: "/missions/dark-mode-for-the-dashboard",
    title: "Dark mode for the dashboard",
    status: {
      icon: "circle-dot",
      tone: "progress",
      words: "Working",
      live: true,
    },
  },
  {
    name: "api#45",
    to: "/reviews/api/45",
    title: "api#45 · Paginate the invoices endpoint",
    icon: "pr",
  },
  {
    name: "records:adr/0003",
    to: "/documents/records/adr/0003-an-invoice-freezes-when-it-is-sent.md",
    title: "ADR 0003: An invoice freezes when it is sent",
    icon: "file",
  },
];
