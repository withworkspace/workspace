import { scope } from "../keymap/commands";

// viewerKeys are the picture viewer's (Viewer.tsx binds them, while it's
// open: nobody else's keys work then).
export const viewerKeys = scope("viewer", "Picture viewer", {
  close: { title: "Close", keys: "Escape" },
  zoomIn: { title: "Zoom in", keys: ["+", "="], row: "Zoom in / out" },
  zoomOut: { title: "Zoom out", keys: "-", row: "Zoom in / out" },
  fit: { title: "Fit to the window", keys: "0" },
  actual: { title: "Actual size", keys: "1" },
  left: { title: "Move left", keys: ["h", "ArrowLeft"], row: "Move the picture" },
  down: { title: "Move down", keys: ["j", "ArrowDown"], row: "Move the picture" },
  up: { title: "Move up", keys: ["k", "ArrowUp"], row: "Move the picture" },
  right: { title: "Move right", keys: ["l", "ArrowRight"], row: "Move the picture" },
});

// quietKeys has no commands: bound as a modal (useCommands(quietKeys, {},
// { modal: true })), it's what's open over the page keeping the page's
// keys off while it is (a dialog, a reply being written).
export const quietKeys = scope("quiet", "Dialogs", {});
