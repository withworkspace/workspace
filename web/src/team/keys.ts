import { scope } from "../keymap/commands";

// teamKeys are Team's keys (Team.tsx binds them).
export const teamKeys = scope("team", "Team", {
  next: { title: "Next", keys: ["j", "ArrowDown"], row: "Next / previous", moves: true },
  prev: { title: "Previous", keys: ["k", "ArrowUp"], row: "Next / previous", moves: true },
  first: { title: "First", keys: "g", row: "First / last", moves: true },
  last: { title: "Last", keys: "G", row: "First / last", moves: true },
  toggle: { title: "Expand or collapse", keys: "Enter" },
  open: { title: "Open: review here, or on GitHub", keys: "o" },
  summarise: { title: "Summarise, or retry", keys: "w" },
  summariseAll: { title: "Summarise all shown", keys: "W" },
  worthKnowing: { title: "Worth knowing only, or everything", keys: "n" },
  catchUp: { title: "Catch up", keys: "c" },
  allChanges: { title: "Show all changes", keys: "Escape" },
  byPerson: { title: "By person, or by day", keys: "p" },
  today: { title: "Today", keys: "1", row: "Today / 7 days / all" },
  week: { title: "7 days", keys: "2", row: "Today / 7 days / all" },
  all: { title: "All", keys: "3", row: "Today / 7 days / all" },
  bots: { title: "Show or hide bots", keys: "a" },
  filter: { title: "Filter by text, repo or person", keys: "/" },
  refresh: { title: "Refresh", keys: "r" },
});
