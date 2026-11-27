import { scope } from "../keymap/commands";

// reviewsKeys are the Reviews list's keys (Reviews.tsx binds them).
export const reviewsKeys = scope("reviews", "Reviews", {
  next: { title: "Next pull request", keys: ["j", "ArrowDown"], row: "Next / previous pull request", moves: true },
  prev: { title: "Previous pull request", keys: ["k", "ArrowUp"], row: "Next / previous pull request", moves: true },
  first: { title: "First", keys: "g", row: "First / last", moves: true },
  last: { title: "Last", keys: "G", row: "First / last", moves: true },
  open: { title: "Open review", keys: "Enter" },
  github: { title: "Open on GitHub", keys: "o" },
  fold: { title: "Expand / collapse stack", keys: "l" },
  filter: { title: "Filter by text, repo or author", keys: "/" },
  refresh: { title: "Refresh", keys: "r" },
});

// prReviewKeys are a pull request's own, beside the review's
// (PRReview.tsx gives them to ReviewView, which binds them).
export const prReviewKeys = scope("reviews-pr", "Pull request", {
  below: { title: "Layer below", keys: "[", row: "Layer below / above" },
  above: { title: "Layer above", keys: "]", row: "Layer below / above" },
  whole: { title: "Whole stack / single layer", keys: "w" },
  askBar: { title: "Ask bar", keys: "/" },
  askAll: { title: "Ask about the pull request", keys: "A" },
  github: { title: "Open on GitHub", keys: "o" },
  dock: { title: "Open here", keys: "O" },
  back: { title: "Clear selection, or back to Reviews", keys: "Escape" },
});

// prSubmitKeys are the submit dialog's, for a stack (PRSubmitDialog.tsx),
// beside the verdicts' (review/keys.ts, verdictKeys).
export const prSubmitKeys = scope("reviews-submit", "Submit stack review", {
  approveAll: { title: "Approve every layer", keys: "A" },
  before: { title: "Layer before", keys: "[", row: "Pick a layer" },
  after: { title: "Layer after", keys: "]", row: "Pick a layer" },
});
