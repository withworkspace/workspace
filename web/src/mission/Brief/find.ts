import { readsAs } from "../../ask/ask";
import type { DocId } from "./docState";

// Where the Brief tab's code finds what's drawn: the documents say which
// they are (data-brief-doc, MissionBriefDoc) and where their heads are
// (data-brief-doc-head), and where the planner's asked says so
// (data-brief-ask). What's read in a document is the reader's or the
// editor's, which say markdown.

const text = "[data-brief-doc] .markdown";

// docsIn is the documents drawn in an element, as their elements.
export const docsIn = (within: HTMLElement) => [...within.querySelectorAll<HTMLElement>("[data-brief-doc]")];

// docOf is which document an element of docsIn is.
export const docOf = (el: HTMLElement) => el.dataset.briefDoc as DocId;

// headingsIn is a document's headings, as they're drawn.
export const headingsIn = (doc: HTMLElement) => [...doc.querySelectorAll<HTMLElement>(".markdown :is(h1, h2, h3)")];

// The step a plan's nth is, in the plan as it's drawn: its first numbered
// list is its steps.
export const stepIn = (docs: HTMLElement | null, n: number) => docs?.querySelector<HTMLElement>(`[data-brief-doc="plan"] .markdown > ol:first-of-type > li:nth-child(${n})`) ?? null;

// firstHead is the head of the first document in the pane: where the
// documents' text starts.
export const firstHead = (main: HTMLElement) => main.querySelector<HTMLElement>("[data-brief-doc] [data-brief-doc-head]");

// writing says whether an element is in a document or where the planner's
// asked: somewhere typed in.
export const writing = (el: Element | null) => !!el?.closest("[data-brief-doc], [data-brief-ask]");

// flash marks an element a moment (a step gone to from the outline):
// MissionBriefDoc's rule draws it.
export function flash(el: HTMLElement) {
  el.removeAttribute("data-brief-flash");
  void el.offsetWidth;
  el.setAttribute("data-brief-flash", "");
}

// pickedIn is the passage picked out of the documents, as it reads; none
// for a selection that isn't in them, or is too little to be one.
export function pickedIn(docs: HTMLElement | null): string | undefined {
  const sel = getSelection();
  if (!docs || !sel || sel.isCollapsed || !sel.anchorNode) return undefined;
  const at = sel.anchorNode.nodeType === Node.ELEMENT_NODE ? (sel.anchorNode as Element) : sel.anchorNode.parentElement;
  if (!at || !docs.contains(at) || !at.closest(text)) return undefined;
  const passage = readsAs(sel.toString());
  return passage.length >= 3 ? passage : undefined;
}
