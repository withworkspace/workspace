import { scope } from "../../keymap/commands";

// listKeys are the missions' list's keys (Missions.tsx binds them).
export const listKeys = scope("mission-list", "Missions", {
  next: { title: "Next mission", keys: ["j", "ArrowDown"], row: "Next / previous mission", moves: true },
  prev: { title: "Previous mission", keys: ["k", "ArrowUp"], row: "Next / previous mission", moves: true },
  first: { title: "First", keys: "g", row: "First / last", moves: true },
  last: { title: "Last", keys: "G", row: "First / last", moves: true },
  open: { title: "Open", keys: ["Enter", "o"] },
  past: { title: "Show or hide past missions", keys: "A" },
});
