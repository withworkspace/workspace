import { scope } from "../keymap/commands";

// pageConversationKeys are the keys of a page's conversation, where the
// page offers one (PageConversation.tsx binds them, with everywhere's
// ⌘J to go to it: shell/keys.ts).
export const pageConversationKeys = scope("page-conversation", "Page conversation", {
  ask: { title: "Ask about the page", keys: "/" },
});
