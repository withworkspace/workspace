import type { Diff, DismissWhy, PassFinding, Review as ReviewData, ReviewState } from "../api";
import type { KeyMap } from "../keys";
import type { DiffAt } from "../ui/DiffLine";
import type { Order } from "./files";

// What a review page is given, and what it's of: a mission's capsule
// (Review.tsx), or a pull request (reviews/PRReview.tsx).

// ViewKey names a review's views: what's read beside the changes, and the
// changes. One shows at a time, in this order down the side.
export type ViewKey = "overview" | "guide" | "discussion" | "passes" | "changes";

// ReviewPart is one of the views read beside the changes. Its body may
// want to send the cursor to something in the diff (a pass's finding, a
// comment): that shows the Changes view. seen, where there is one, is kept
// each time it's looked at (how much had been said, say), and news says
// what's new since, of the seen that was kept.
export interface ReviewPart {
  key: Exclude<ViewKey, "changes">;
  mark?: React.ReactNode; // how it stands, beside its name in the list
  count?: number; // how much is in it
  seen?: string;
  news?: (seen: string) => string | undefined;
  body: React.ReactNode | ((go: { toFinding: (pass: string, key: string) => boolean; toComment: (id: string) => boolean }) => React.ReactNode);
}

// ReviewDiff is a diff to review: a capsule's, or a pull request's, which
// may also say which files' changes are as you last reviewed them (after a
// rebase, when the diff is all of it).
export type ReviewDiff = Diff & { rebased?: boolean; unchanged?: string[] };

// ShownFinding is a pass's finding to show in the diff: the pass's name
// and what it's called, and the finding.
export interface ShownFinding {
  pass: string;
  title: string;
  finding: PassFinding;
}

// Selection is lines picked in the diff: to comment on, or to ask about.
export interface Selection {
  path: string;
  side: "old" | "new";
  line: number;
  endLine: number;
  commit: string; // the commit those lines are at
}

// ReviewSource is what's being reviewed, and where its diff and its
// reviews come from: a mission's capsule, or a pull request.
export interface ReviewSource {
  api: string; // where its diff, files and reviews are read and written
  title: string; // the page's
  // Its bar: where it stands (ReviewStand), its name, and after it what's
  // said of it (what it's in, who by: about) and whether it can go in
  // (health), which come either side of how many commits it's of.
  stand?: React.ReactNode;
  name: React.ReactNode;
  about?: React.ReactNode;
  health?: React.ReactNode;
  back: { to: string; label: string };
  // headNow is where it is now: a diff of an earlier commit offers to
  // reload. updated changes when its reviews may have, to read them again.
  headNow?: string;
  updated?: string;
  // sinceOf says what "since your last review" is of (a name for it), or
  // undefined when there's nothing since; getDiff reads the diff, of that
  // or (since undefined) of everything. diffKey changes when the diff does.
  sinceOf: (latest?: ReviewData) => string | undefined;
  getDiff: (since?: string) => Promise<ReviewDiff>;
  diffKey?: string;
  // opened is its reviews and its diff (as getDiff gives it, of what
  // sinceOf says), read already: it opens on them, not on nothing.
  opened?: { rs: ReviewState; diff: ReviewDiff };
  sinceName?: (latest?: ReviewData) => string; // "r2", "your last review"
  movedWords: string; // it has new commits since the diff loaded
  empty: string; // there's nothing in the diff
  browse?: string; // where its files are browsed
  // readOnly is why comments can't be made here, when they can't.
  readOnly?: string;
  side?: React.ReactNode; // above the list of files
  // At the bar's end, before where else it opens: what else is done to it.
  actions?: React.ReactNode;
  // Where else it opens, as MenuItems in the bar's Open menu (GitHub,
  // docked here): browse comes last there, where it has files to browse.
  opens?: React.ReactNode;
  notices?: React.ReactNode; // under the bar
  // views are what's read beside the changes (a pull request's
  // description, its guide, what's been said on it, its passes), each a
  // view of its own above the changes in the list down the side, in the
  // order given. The page opens on the first.
  views?: ReviewPart[];
  // order is an order to read the files in that isn't their folders' (a
  // guide's); onOrder switches between the two, where there's one.
  order?: Order;
  onOrder?: (guided: boolean) => void;
  // guideNotes are the guide's line on each file, by path: what the
  // change does to it. They're said on the file in either order.
  guideNotes?: Record<string, string>;
  // followups are what a follow-up said of your comments, by the comment
  // (its thread): whether what's been pushed since deals with it. It's
  // said on the comment.
  followups?: Record<string, { kind: string; text: string; blocks?: boolean }>;
  // findings are what passes found that's still open, to show on the
  // diff's lines; onFinding takes one into the draft or dismisses it, and
  // answers once that's done. An ask about one goes to onAsk, with what
  // was found said first.
  findings?: ShownFinding[];
  onFinding?: (pass: string, key: string, what: "taken" | "dismissed", why?: DismissWhy) => Promise<unknown>;
  // threads are comments kept somewhere else (a pull request's, on
  // GitHub), shown as a submitted review's; onThread answers one, and
  // gives them back as they then are. With them, your own submitted
  // comments aren't drawn from the reviews kept here: they're among them.
  threads?: ReviewData;
  onThread?: (id: string, what: "reply" | "resolve" | "reopen", body: string) => Promise<unknown>;
  // submit draws the dialog that ends the review; done is called with the
  // reviews as they then are.
  submit: (ctx: { draft: ReviewData | null; diff: ReviewDiff; onClose: () => void; onDiscard: () => void; done: (next: ReviewState) => void }) => React.ReactNode;
  submitted: (r: ReviewData, state: ReviewState) => React.ReactNode; // said once it's submitted
  // onAsk asks an agent about the selected lines (beside Comment), or with
  // none, about the whole of it; with a pass's finding, what it found.
  onAsk?: (sel?: Selection, finding?: AskedFinding) => void;
  // foot stands over the foot of the changes, outside their scroll, so
  // it stays put however far down they go (a pull request's ask bar);
  // dismiss drops what it has open over them (esc), saying whether it had
  // anything.
  foot?: React.ReactNode;
  dismiss?: () => boolean;
  keys?: KeyMap; // more keys, the page's own
}

// AskedFinding is a pass's finding asked about: the pass, by its title,
// and what it found.
export interface AskedFinding {
  pass: string;
  kind: string;
  text: string;
}

// NavAt is where a file's sides are read, for code navigation: the
// diff's base and head, and the file's path at each.
export type NavAt = DiffAt;
