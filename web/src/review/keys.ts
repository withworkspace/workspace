import { scope } from "../keymap/commands";

// reviewKeys are a review's keys, a mission's or a pull request's
// (ReviewView.tsx binds them). Those of the diff are bound while the
// changes show.
export const reviewKeys = scope("review", "Review", {
  view1: { title: "View 1", keys: "1", row: "Overview, Guide, Discussion, Passes, Changes" },
  view2: { title: "View 2", keys: "2", row: "Overview, Guide, Discussion, Passes, Changes" },
  view3: { title: "View 3", keys: "3", row: "Overview, Guide, Discussion, Passes, Changes" },
  view4: { title: "View 4", keys: "4", row: "Overview, Guide, Discussion, Passes, Changes" },
  view5: { title: "View 5", keys: "5", row: "Overview, Guide, Discussion, Passes, Changes" },
  nextLine: { title: "Next line", keys: ["j", "ArrowDown"], row: "Next / previous line", moves: true },
  prevLine: { title: "Previous line", keys: ["k", "ArrowUp"], row: "Next / previous line", moves: true },
  nextChange: { title: "Next change", keys: "n", row: "Next / previous change", moves: true },
  prevChange: { title: "Previous change", keys: "p", row: "Next / previous change", moves: true },
  nextFile: { title: "Next file", keys: "J", row: "Next / previous file", moves: true },
  prevFile: { title: "Previous file", keys: "K", row: "Next / previous file", moves: true },
  halfDown: { title: "Down half a page", keys: "mod+d", row: "Down / up half a page", moves: true },
  halfUp: { title: "Up half a page", keys: "mod+u", row: "Down / up half a page", moves: true },
  first: { title: "First", keys: "g", row: "First / last", moves: true },
  last: { title: "Last", keys: "G", row: "First / last", moves: true },
  findFile: { title: "Jump to a file", keys: "t" },
  comment: { title: "Comment on line or selection", keys: "c" },
  ask: { title: "Ask about line or selection", keys: "a" },
  selectDown: { title: "Select lines down", keys: "shift+ArrowDown", row: "Select lines (or drag line numbers)" },
  selectUp: { title: "Select lines up", keys: "shift+ArrowUp", row: "Select lines (or drag line numbers)" },
  selectFrom: { title: "Select lines from here, then move", keys: "V" },
  activate: { title: "Reply to or edit comment; take finding; expand hidden lines or file", keys: "Enter" },
  resolve: { title: "Resolve or reopen comment; else mark file viewed", keys: "x" },
  remove: { title: "Delete draft comment; dismiss finding (then 1, 2 or 3 for why)", keys: "D" },
  nextComment: { title: "Next comment", keys: "N", row: "Next / previous comment" },
  prevComment: { title: "Previous comment", keys: "P", row: "Next / previous comment" },
  submit: { title: "Submit review", keys: "s" },
  since: { title: "Since last review / all changes", keys: "i" },
  more: { title: "Show 20 more lines around this change", keys: "e" },
  split: { title: "Unified / side by side", keys: "u" },
  reload: { title: "Reload", keys: "r" },
  back: { title: "Clear selection, or back to mission", keys: "Escape" },
});

// reviewNavKeys go to what a name in the diff is, and back (CodeNav.tsx
// says what they do; ReviewView.tsx binds them while the changes show).
export const reviewNavKeys = scope("review-nav", "Definitions", {
  definition: { title: "Go to definition on selected line", keys: "d" },
  back: { title: "Back from definition", keys: "b" },
  forward: { title: "Forward again", keys: "B" },
});

// reviewPanelKeys are those of the panel a definition or its uses open
// in (NavPanel.tsx, UsesPanel.tsx): while it's open, the keys are its own.
export const reviewPanelKeys = scope("review-panel", "Definition or uses", {
  down: { title: "Down", keys: ["j", "ArrowDown"], row: "Down / up", moves: true },
  up: { title: "Up", keys: ["k", "ArrowUp"], row: "Down / up", moves: true },
  halfDown: { title: "Down half a page", keys: "mod+d", row: "Down / up half a page" },
  halfUp: { title: "Up half a page", keys: "mod+u", row: "Down / up half a page" },
  go: { title: "Go to it", keys: "Enter" },
  open: { title: "Open current version in Files", keys: "o" },
  back: { title: "Back", keys: "b" },
  close: { title: "Close", keys: "Escape" },
});

// reviewPickKeys are those of what's picked from over the diff: a card of
// what a name is, pinned (CodeNavCard.tsx), or which name on a line to
// follow (CodeNavPicker.tsx). While it's open, the keys are its own.
export const reviewPickKeys = scope("review-pick", "Pick", {
  next: { title: "Next", keys: ["j", "ArrowDown"], row: "Next / previous", moves: true },
  prev: { title: "Previous", keys: ["k", "ArrowUp"], row: "Next / previous", moves: true },
  go: { title: "Go to it", keys: "Enter" },
  follow: { title: "Go to the definition", keys: "d" },
  close: { title: "Close", keys: "Escape" },
});

// reviewDismissKeys say why a finding's dismissed (ReviewDismissAs.tsx).
export const reviewDismissKeys = scope("review-dismiss", "Dismiss as", {
  wrong: { title: "Wrong", keys: "1" },
  already: { title: "Said already", keys: "2" },
  minor: { title: "Not worth it", keys: "3" },
  cancel: { title: "Cancel", keys: "Escape" },
});

// verdictKeys are a review's verdicts as it's submitted (ReviewVerdicts.tsx
// says them; ReviewSubmit.tsx and the pull request's dialog bind them).
export const verdictKeys = scope("review-verdict", "Submit review", {
  approve: { title: "Approve", keys: "a" },
  changes: { title: "Request changes", keys: "c" },
  comment: { title: "Comment", keys: "m" },
  submit: { title: "Submit", keys: "mod+Enter", fields: true },
  // The dialog's own: it closes on Escape.
  cancel: { title: "Cancel", keys: "Escape", fixed: true },
});

// reviewLandKeys land a capsule's work on its default branch, in a repo
// without pull requests (LandBar.tsx).
export const reviewLandKeys = scope("review-land", "Land", {
  ask: { title: "Land on default branch (repos without pull requests)", keys: "L" },
  land: { title: "Land", keys: "Enter" },
  cancel: { title: "Cancel", keys: "Escape" },
});
