import { scope } from "../keymap/commands";

// everywhereKeys are the keys on every page (useGlobalKeys, App.tsx, and
// the parts beside: FocusKeys, usePeekKey, the page's conversation). The
// Keys dialog says them after the page's own; those marked frame work in a
// plugin's page as they do here (keys.ts, frameKeys).
export const everywhereKeys = scope("everywhere", "Everywhere", {
  find: { title: "Find anything", keys: "mod+k", fields: true, frame: true },
  findFile: { title: "Find a file", keys: "mod+p", fields: true, frame: true },
  findCode: { title: "Search in files (⌫ for all repos)", keys: "mod+shift+f", fields: true },
  goMissions: { title: "Go to Missions", keys: "g m", frame: true },
  goConversations: { title: "Go to Conversations", keys: "g c", frame: true },
  goDocuments: { title: "Go to Documents", keys: ["g d", "g p"], frame: true },
  goArtifacts: { title: "Go to Artifacts", keys: "g a", frame: true },
  goReviews: { title: "Go to Reviews", keys: "g r", frame: true },
  goTeam: { title: "Go to Team", keys: "g t", frame: true },
  goFiles: { title: "Go to Files", keys: "g f", frame: true },
  goOrchestrator: { title: "Go to Orchestrator", keys: "g o", frame: true },
  conversation: { title: "The page's conversation", keys: "mod+j", fields: true, frame: true },
  pin: { title: "Pin or unpin in Focus", keys: "." },
  peek: { title: "Peek at what's under the pointer; again to close", keys: "z" },
  converse: { title: "Start a conversation beside the page", keys: "C", frame: true },
  beside: { title: "Open or close side panel", keys: "\\" },
  help: { title: "Keyboard shortcuts", keys: "?", frame: true },
  back: { title: "Close, or go back", keys: "Escape", fixed: true },
});
