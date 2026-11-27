import { scope } from "../keymap/commands";

// The keys of Documents (Documents.tsx) and of a space's page
// (SpacePage.tsx). Documents' j, k and ⏎ move among what's found while
// there's a search, and among the spaces and pages otherwise: one or the
// other is bound.

export const documentsKeys = scope("documents", "Documents", {
  newPage: { title: "New page", keys: "n" },
  newAsKind: { title: "New page from a document kind", keys: "N" },
  search: { title: "Search documents", keys: "/" },
  next: { title: "Next space or page", keys: ["j", "ArrowDown"], row: "Move between spaces and pages", moves: true },
  prev: { title: "Previous space or page", keys: ["k", "ArrowUp"], row: "Move between spaces and pages", moves: true },
  open: { title: "Open", keys: ["Enter", "o"] },
  nextHit: { title: "Next found", keys: "j", row: "Move between what's found", moves: true },
  prevHit: { title: "Previous found", keys: "k", row: "Move between what's found", moves: true },
  openHit: { title: "Open what's found", keys: ["Enter", "o"], row: "Open" },
  clearSearch: { title: "Clear search", keys: "Escape" },
});

export const spaceKeys = scope("documents-space", "Space", {
  next: { title: "Next document", keys: ["j", "ArrowDown"], row: "Move between documents", moves: true },
  prev: { title: "Previous document", keys: ["k", "ArrowUp"], row: "Move between documents", moves: true },
  open: { title: "Read", keys: ["Enter", "o"] },
  search: { title: "Search the space", keys: "/" },
  newHere: { title: "New page here", keys: "n" },
  back: { title: "Clear search, or back to Documents", keys: "Escape" },
});
