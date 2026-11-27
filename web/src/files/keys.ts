import { scope } from "../keymap/commands";

// The keys of Files: a repo's or a capsule's files (Browser/FileBrowser.tsx,
// for the Files page and a mission's explorer), the switch between a
// repo's ground and capsules (Repos/WorktreeSwitch.tsx), and a document
// read (DocPage.tsx). Going to a name's definition and back are the
// review's (review/keys.ts, reviewNavKeys), bound where a file's lines
// are (Browser/FileCode.tsx).

export const filesKeys = scope("files", "Files", {
  find: { title: "Find a file", keys: ["t", "mod+p"] },
  next: { title: "Next in the tree", keys: ["j", "ArrowDown"], row: "Next / previous in the tree", moves: true },
  prev: { title: "Previous in the tree", keys: ["k", "ArrowUp"], row: "Next / previous in the tree", moves: true },
  enter: { title: "Open file, or toggle folder; on the open markdown, open it as document", keys: "Enter", row: "Open file, or toggle folder" },
  open: { title: "Open file, or toggle folder", keys: "o", row: "Open file, or toggle folder" },
  closeFolder: { title: "Close folder", keys: ["h", "ArrowLeft"], row: "Close / open folder" },
  openFolder: { title: "Open folder", keys: ["l", "ArrowRight"], row: "Close / open folder" },
  changed: { title: "Changed files, or all files (capsules)", keys: "c" },
  prevChanged: { title: "Previous changed file", keys: "[", row: "Previous / next changed file" },
  nextChanged: { title: "Next changed file", keys: "]", row: "Previous / next changed file" },
  definitionClick: { title: "Go to definition or usages", keys: "⌘-click", fixed: true },
  markdown: { title: "Markdown: rendered or source", keys: "m" },
  review: { title: "Review changes (mission capsules)", keys: "v" },
  copyPath: { title: "Copy path", keys: "y" },
  panes: { title: "Panes: which are shown beside the file", keys: "p" },
  reload: { title: "Reload", keys: "r" },
  close: { title: "Close file", keys: "Escape" },
  backToMission: { title: "Back to mission", keys: "Escape" },
});

export const switchKeys = scope("files-switch", "Capsule or ground", {
  open: { title: "Switch capsule or ground", keys: "w" },
  next: { title: "Next", keys: ["j", "ArrowDown"], row: "Next / previous", moves: true },
  prev: { title: "Previous", keys: ["k", "ArrowUp"], row: "Next / previous", moves: true },
  go: { title: "Switch to it", keys: "Enter" },
  close: { title: "Close", keys: "Escape" },
});

export const docKeys = scope("files-doc", "Document", {
  edit: { title: "Edit", keys: "e" },
  halfDown: { title: "Down half a page", keys: "mod+d", row: "Down / up half a page" },
  halfUp: { title: "Up half a page", keys: "mod+u", row: "Down / up half a page" },
  top: { title: "Top", keys: "g", row: "Top / bottom" },
  bottom: { title: "Bottom", keys: "G", row: "Top / bottom" },
  contents: { title: "Show or hide contents", keys: "t" },
  prevDoc: { title: "Previous document", keys: "[", row: "Previous / next document" },
  nextDoc: { title: "Next document", keys: "]", row: "Previous / next document" },
  up: { title: "Open folder; again to go up", keys: "u" },
  askBar: { title: "Go to ask bar", keys: "/" },
  ask: { title: "Ask (about selection, if any)", keys: "a" },
  files: { title: "Open in Files", keys: "o" },
  reload: { title: "Reload", keys: "r" },
  back: { title: "Back, or close", keys: "Escape", row: "Back, or close" },
  backAlso: { title: "Back", keys: "h", row: "Back, or close" },
});
