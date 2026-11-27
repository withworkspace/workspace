import { scope } from "../../keymap/commands";

// findKeys are Find's own, those it binds (Palette.tsx) beside what its
// box does with the keys typed in it.
export const findKeys = scope("find", "Find", {
  back: { title: "Back out of what Find's doing, or close it", keys: "Escape" },
});
