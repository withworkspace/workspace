// A document's contents as the reader shows them: its h2 and h3 headings,
// which of them you're in, and how far through it you are. What the reader
// works out from where things are on the screen, kept apart from drawing it
// so it can be tested.

// Section is one of a document's headings the contents list: its name (the
// anchor GitHub gives it, which a link to it ends in), its level, and its
// words.
export interface Section {
  anchor: string;
  level: 2 | 3;
  text: string;
}

// The reader's text needs about this much width beside a rail of contents
// (and the gap between them) before the rail fits; narrower, the contents
// fold into the pill.
export const textMin = 600;
export const railWidth = 200;
export const railGap = 40;
export const railRoom = textMin + railGap + railWidth;

// A heading is the one you're in once it's this near the top of the
// reader, or above it.
export const sectionLine = 80;

// sectionAt is which section you're in: the last whose heading has reached
// the line (tops are each heading's distance below the top of the reader,
// in order), or -1 before the first.
export function sectionAt(tops: number[], line = sectionLine): number {
  let at = -1;
  for (let i = 0; i < tops.length; i++) {
    if (tops[i] > line) break;
    at = i;
  }
  return at;
}

// progress is how far through the reader you've scrolled, as a whole
// percentage: all of it when there's nothing to scroll.
export function progress(scrollTop: number, scrollHeight: number, clientHeight: number): number {
  const max = scrollHeight - clientHeight;
  if (max <= 0) return 100;
  return Math.min(100, Math.max(0, Math.round((scrollTop / max) * 100)));
}
