import type { DocHit, SpaceDoc } from "../api";

// A list's rows can be picked (a space's page shows the one picked beside
// its list): a click picks it, a double click (or ⏎) opens it, and a
// modified click is the browser's. A search's hit says where in it.
export type Picking = {
  picked?: string; // its path
  onPick: (d: SpaceDoc) => void;
  onOpen: (d: SpaceDoc) => void;
  hits?: Map<string, DocHit>;
};

// pickOn is a row's click, where its list's rows are picked: a plain click
// picks it; a modified one opens it as a link does.
export const pickOn = (picking: Picking, d: SpaceDoc) => (e: React.MouseEvent) => {
  if (e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
  e.preventDefault();
  if (e.detail < 2) picking.onPick(d);
};
