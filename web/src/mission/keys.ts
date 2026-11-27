import { type Command, scope } from "../keymap/commands";

// missionKeys are a mission's page's keys (Mission.tsx binds them). Its
// tabs' parts bind their own: Brief's (Brief/keys.ts) and Verify's
// (Verify/keys.ts), while each is open.
const tabs = "Summary, Brief, Work, Verify, Log, Talk, Decisions";
export const missionKeys = scope("mission", "Mission", {
  summary: { title: "Summary tab", keys: "1", row: tabs },
  brief: { title: "Brief tab", keys: "2", row: tabs },
  work: { title: "Work tab", keys: "3", row: tabs },
  verify: { title: "Verify tab", keys: "4", row: tabs },
  log: { title: "Log tab", keys: "5", row: tabs },
  talk: { title: "Talk tab", keys: "6", row: tabs },
  decisions: { title: "Decisions tab", keys: "7", row: tabs },
  next: { title: "Next item", keys: ["j", "ArrowDown"], row: "Next / previous item", moves: true },
  prev: { title: "Previous item", keys: ["k", "ArrowUp"], row: "Next / previous item", moves: true },
  open: { title: "Open", keys: "Enter" },
  review: { title: "Review capsule / read replies", keys: "v" },
  openLink: { title: "Open link", keys: "o" },
  files: { title: "Browse files", keys: "f" },
  edit: { title: "Edit brief and plan", keys: "e" },
  details: { title: "Toggle details", keys: "s" },
  reload: { title: "Reload", keys: "r" },
  back: { title: "Close, or back to missions", keys: "Escape" },
  home: { title: "Back to missions", keys: "h" },
});

// tabKeys is the command that goes to each of a mission's tabs, by tab.
export const tabKeys: Record<string, Command> = {
  summary: missionKeys.summary,
  brief: missionKeys.brief,
  work: missionKeys.work,
  verify: missionKeys.verify,
  log: missionKeys.log,
  talk: missionKeys.talk,
  decisions: missionKeys.decisions,
};
