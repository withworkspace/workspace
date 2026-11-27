import { create } from "zustand";

// MergeOffer is what can be done to merge a pull request, as the page
// that last showed it read it: merged now, left to merge by itself once
// it can, or (on) stopped doing so; at the commit it showed (head). A
// stack's is its top's: it merges whole.
export interface MergeOffer {
  offer: "now" | "ready" | "on";
  head?: string;
  // The stack it merges, and how many layers: offered on any of them.
  stack?: number;
  layers?: number;
  // Where the merge is asked of: the stack's top, or the pull request.
  repo: string;
  number: number;
}

// The merge offers of pull requests a page has shown, by repo#n, for
// Find's acts (shell/acts.ts), which know a pull request only by its name.
// A page sets what it read, and takes it away once it's nothing to offer.
export const useMergeOffers = create<Record<string, MergeOffer>>()(() => ({}));

export function offerMerge(ref: string, offer: MergeOffer | undefined) {
  useMergeOffers.setState((s) => {
    const was = s[ref];
    if (!offer) {
      if (!was) return s;
      const { [ref]: _gone, ...rest } = s;
      return rest;
    }
    if (was && was.offer === offer.offer && was.head === offer.head && was.number === offer.number) return s;
    return { ...s, [ref]: offer };
  }, true);
}
