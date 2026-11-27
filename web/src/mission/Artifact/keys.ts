import { scope } from "../../keymap/commands";

// artifactKeys are the keys of one of a mission's documents
// (ArtifactPage.tsx binds them): the page's, after the document's own (a
// document that can be commented on has its own j, k and esc, to move
// through it: annotate's). Not while it's being edited, when the editor
// has the keys.
export const artifactKeys = scope("mission-artifact", "Document", {
  down: { title: "Scroll down", keys: ["j", "ArrowDown"], row: "Scroll down / up (in code)" },
  up: { title: "Scroll up", keys: ["k", "ArrowUp"], row: "Scroll down / up (in code)" },
  edit: { title: "Edit", keys: "e" },
  halfDown: { title: "Down half a page", keys: "mod+d", row: "Down / up half a page" },
  halfUp: { title: "Up half a page", keys: "mod+u", row: "Down / up half a page" },
  top: { title: "Top", keys: "g", row: "Top / bottom" },
  bottom: { title: "Bottom", keys: "G", row: "Top / bottom" },
  open: { title: "Open in repo", keys: "o" },
  reload: { title: "Reload", keys: "r" },
  back: { title: "Back", keys: ["Escape", "h"] },
});
