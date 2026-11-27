import { scope } from "../keymap/commands";

// settingsKeys are the Settings page's keys (Settings.tsx binds them).
export const settingsKeys = scope("settings", "Settings", {
  find: { title: "Find a setting", keys: "/" },
});
