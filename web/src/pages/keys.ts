import { scope } from "../keymap/commands";

// pageKeys are a page's, being written (Page.tsx): the editor's own, so
// they're said, and can't be set to others.
export const pageKeys = scope("pages", "Page", {
  fill: { title: "Ask an agent to fill in here", keys: ">> then ⏎", fixed: true },
  ask: { title: "Ask a question, answered beside the page", keys: ">? then ⏎", fixed: true },
  change: { title: "In a cell: change or see sources. Elsewhere: change the text", keys: "mod+Enter", fixed: true },
  closeCell: { title: "Close a cell's panel", keys: "Escape", fixed: true },
});
