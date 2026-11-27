import { scope } from "../../keymap/commands";

// briefKeys are the Brief tab's keys (Brief.tsx binds them while it's
// open): its changes to read, the planner, and what's confirmed.
export const briefKeys = scope("mission-brief", "Brief", {
  next: { title: "Next change", keys: "j", row: "Next / previous change", moves: true },
  prev: { title: "Previous change", keys: "k", row: "Next / previous change", moves: true },
  accept: { title: "Accept change; with text selected, ask the planner", keys: "a", row: "Accept / decline change; with text selected, accept asks the planner" },
  decline: { title: "Decline change", keys: "d", row: "Accept / decline change; with text selected, accept asks the planner" },
  ask: { title: "Ask the planner", keys: "/" },
  planner: { title: "Show or hide planner", keys: "p" },
  confirm: { title: "Confirm, accept or save (in the editor too)", keys: "mod+Enter" },
  back: { title: "Leave the editor, or back to missions", keys: "Escape" },
});
