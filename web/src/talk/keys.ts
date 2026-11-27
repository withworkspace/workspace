import { scope } from "../keymap/commands";

// talksKeys are a list of conversations' keys (TalkList.tsx binds them,
// on Conversations).
export const talksKeys = scope("talks", "Conversations", {
  start: { title: "New conversation", keys: "n" },
  next: { title: "Next", keys: ["j", "ArrowDown"], row: "Next / previous", moves: true },
  prev: { title: "Previous", keys: ["k", "ArrowUp"], row: "Next / previous", moves: true },
  open: { title: "Open", keys: ["Enter", "o"] },
  resolved: { title: "Show or hide resolved", keys: "A" },
});

// talkKeys are a conversation's keys (TalkPage.tsx binds them). Send is
// its box's own (ui/sendKey.ts: Settings › Keyboard, Send with).
export const talkKeys = scope("talk", "Conversation", {
  send: { title: "Send", keys: "mod+Enter", fixed: true },
  reply: { title: "Reply", keys: "r" },
  resolve: { title: "Resolve or reopen", keys: "x" },
  back: { title: "Back to conversations", keys: "Escape" },
});
