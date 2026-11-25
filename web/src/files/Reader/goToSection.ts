// A section's heading in the reader's text, by its anchor.
export const headingAt = (article: HTMLElement | null, anchor: string) => article?.querySelector<HTMLElement>(`[data-anchor="${CSS.escape(anchor)}"]`) ?? null;

// goToSection scrolls the reader to a section's heading, and marks it for
// a moment so the eye finds it.
export function goToSection(article: HTMLElement | null, anchor: string) {
  const h = headingAt(article, anchor);
  if (!h) return;
  const still = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  h.scrollIntoView({ block: "start", behavior: still ? "auto" : "smooth" });
  h.classList.remove("flash");
  void h.offsetWidth; // so the mark starts again
  h.classList.add("flash");
}
