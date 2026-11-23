import type { BelowView } from "./Below";

// What Below holds, for the app: nothing yet, so the app has no Below,
// and no toggle for one in its foot. The dock, its place in the window
// and its state are there (Below.tsx, Shell's below, chromeState's
// below); what it's for comes with its first view. Runs are the ones
// meant: what's running and what it came to, as the Runs page lists
// them, and a run's terminal.
//
// A view that reads the workspace is a hook's to make, which is why this
// is one.
const none: BelowView[] = [];

export function useBelowViews(): BelowView[] {
  return none;
}
