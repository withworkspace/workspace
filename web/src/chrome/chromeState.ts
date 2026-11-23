// How one window is arranged: where the sidebar is. It isn't the
// daemon's: it's kept in the browser for the workspace, as the theme and
// the widths are, and two windows may differ.

// Where the sidebar is: open, folded to a rail, or hidden. Hidden, none
// of it is drawn: you get about by the keys (Find, the places' own), and
// the foot's toggle brings it back. On a narrow window it's a drawer: out
// of sight until that toggle opens it over the page.
export type SidebarPlace = "open" | "rail" | "hidden" | "drawer";

// Under this width a window is narrow: there's no room for a sidebar
// beside the page.
export const narrowBelow = 860;
export const narrowQuery = `(max-width: ${narrowBelow}px)`;

export interface ChromeState {
  // The sidebar is hidden. It's yours to choose and holds until you undo
  // it; a page never does it.
  hidden: boolean;
}

// A window nobody has arranged.
export const resting: ChromeState = { hidden: false };

// sidebarPlace is where the sidebar is drawn. A drawer on a narrow
// window, whatever else. Nowhere if you've hidden it; otherwise the page
// says, as it has: a page that wants the room gets the rail.
export function sidebarPlace(s: ChromeState, wantsRoom: boolean, narrow = false): SidebarPlace {
  if (narrow) return "drawer";
  return s.hidden ? "hidden" : wantsRoom ? "rail" : "open";
}

export const hideSidebar = (s: ChromeState, hidden: boolean): ChromeState => ({ ...s, hidden });

// read is the arrangement as it was kept, with anything that's missing or
// isn't what it should be (kept by an earlier ws, or edited) as it rests.
export function read(kept: string | null | undefined): ChromeState {
  let got: { hidden?: unknown } = {};
  try {
    const parsed: unknown = JSON.parse(kept ?? "");
    if (parsed && typeof parsed === "object") got = parsed;
  } catch {
    // Nothing kept, or not what was kept: as it rests.
  }
  return { hidden: got.hidden === true };
}

export const write = (s: ChromeState): string => JSON.stringify(s);
