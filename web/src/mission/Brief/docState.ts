import type { Edit } from "../briefpage";

// The Brief tab's two documents, and where each stands in it.

export type DocId = "brief" | "plan";

// Reading is a document's changes still to settle, and the text they're
// marked in.
export interface Reading {
  pending: Edit[];
  base: string;
}

// DocState is where a document stands in the tab: what it comes to as
// you've left it, whether that's other than what it shows (you edited it,
// or declined a change), the changes still to read with the text they're
// marked in, whether it's the editor (not its changes) that's showing, and
// how its changes are settled.
export interface DocState extends Reading {
  text: () => string;
  changed: boolean;
  editing: boolean;
  settle: (id: string, a: "accept" | "decline") => void;
  all: (a: "accept" | "decline") => void;
}

// Focus is the change in hand, of those still to read.
export interface Focus {
  doc: DocId;
  id: string;
}

// Guard is a document that's read until editing it by hand is asked for
// (the plan, which ws holds to a strict form): what's offered instead is
// having it written again (rewrite; none when there's nobody to ask),
// busy while that's being asked. planned: there's one to rewrite.
export interface Guard {
  rewrite?: () => void;
  busy: boolean;
  planned: boolean;
}
