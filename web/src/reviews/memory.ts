import { create } from "zustand";
import type { ReviewsList } from "../api";

// The Reviews page as you left it, for as long as the window's open: the
// list as last read, so coming back shows it at once while it's read
// again; and what was selected, the stacks folded and the sections
// opened. It's nobody's but the page's, and it outlives the page: a pull
// request's review moves the selection on from here once it's posted
// (Reviews.tsx's reviewed), before the list is back.
interface ReviewsMemory {
  list?: ReviewsList;
  // The row selected, by its key.
  selected: string;
  // The stacks folded, and the folded sections (drafts, bots) opened, by
  // key.
  foldedRows: Record<string, boolean>;
  openSections: Record<string, boolean>;
}

export const useReviewsMemory = create<ReviewsMemory>()(() => ({ selected: "", foldedRows: {}, openSections: {} }));
