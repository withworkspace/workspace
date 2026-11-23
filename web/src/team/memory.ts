import { create } from "zustand";
import type { TeamView } from "../api";

// The Team page as you left it, for as long as the window's open: what
// was last read of each period, so coming back shows it at once while
// it's read again; and what was picked, opened and shown.
interface TeamMemory {
  // What was last read, by period.
  views: Record<string, TeamView>;
  period: string;
  // The row selected, by its id.
  selected: string;
  // What's automated is shown; only the changes with something worth
  // knowing are; they're by person, not by day; the account of what's
  // being worked on is folded.
  showBots: boolean;
  onlyKnow: boolean;
  byPeople: boolean;
  foldSummary: boolean;
  // The changes opened, by key.
  opened: Record<string, boolean>;
}

export const useTeamMemory = create<TeamMemory>()(() => ({ views: {}, period: "7d", selected: "", showBots: false, onlyKnow: false, byPeople: false, foldSummary: false, opened: {} }));
