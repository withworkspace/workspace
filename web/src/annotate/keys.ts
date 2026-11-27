import { scope } from "../keymap/commands";

// annotateKeys are a document's, commented on (Annotate.tsx binds them).
// While a comment's being written they're off: the composer's keys are
// its own (ThreadComposer.tsx).
export const annotateKeys = scope("annotate", "Comments", {
  next: { title: "Next paragraph", keys: ["j", "ArrowDown"], row: "Next / previous paragraph", moves: true },
  prev: { title: "Previous paragraph", keys: ["k", "ArrowUp"], row: "Next / previous paragraph", moves: true },
  extendNext: { title: "Extend to next paragraph", keys: "shift+ArrowDown", row: "Extend to next / previous paragraph" },
  extendPrev: { title: "Extend to previous paragraph", keys: "shift+ArrowUp", row: "Extend to next / previous paragraph" },
  comment: { title: "Comment on paragraphs or selection", keys: "c" },
  nextThread: { title: "Next open thread", keys: "n", row: "Next / previous open thread" },
  prevThread: { title: "Previous open thread", keys: "p", row: "Next / previous open thread" },
  reply: { title: "Reply", keys: "Enter" },
  accept: { title: "Accept suggested edit", keys: "a", row: "Accept / decline suggested edit" },
  decline: { title: "Decline suggested edit", keys: "d", row: "Accept / decline suggested edit" },
  tracked: { title: "Show suggested edits as tracked changes", keys: "v" },
  resolve: { title: "Resolve / reopen thread", keys: "x" },
  since: { title: "Show / hide changes since last comment", keys: "i" },
  approve: { title: "Approve document", keys: "A" },
  back: { title: "Clear what's picked out, or go back", keys: "Escape", row: "Clear what's picked out, or go back" },
  backAlso: { title: "Go back", keys: "h", row: "Clear what's picked out, or go back" },
  kind: { title: "While writing: ask, change or note", keys: "Tab", fixed: true },
});
