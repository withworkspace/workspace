import { scope } from "../../keymap/commands";

// crewKeys are a crew's page's keys (CrewPage.tsx binds them), and what
// the box to them and their terminal do with keys of their own (fixed:
// said here, theirs to bind).
export const crewKeys = scope("crew", "Crew", {
  message: { title: "Message crew", keys: "r" },
  send: { title: "Send", keys: "mod+Enter", fixed: true },
  view: { title: "Terminal / conversation", keys: "t" },
  inTerminal: { title: "In terminal while focused, Esc included", keys: "(any key)", fixed: true },
  copy: { title: "Copy", keys: "mod+c", row: "Copy / paste", fixed: true },
  paste: { title: "Paste", keys: "mod+v", row: "Copy / paste", fixed: true },
  leave: { title: "Leave terminal", keys: "(click outside)", fixed: true },
});
