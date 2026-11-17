import { startAsk, type TalkDetail, type TalkProposal } from "../api";
import { slugger } from "../markdown/links";
import { spoken } from "../talk/proposals";
import type { IconName } from "../ui/Icon";

// What the ask bar (AskBar.tsx) works out, apart from drawing it: what a
// passage picked out is, how what's asked about shows as a chip, whether
// an ask is still being answered, which reply what it proposed came
// with, and where an answer's citation of a section leads.

// Chip is something asked about, as the bar and its card show it: a
// passage (quoted), or lines of a pull request's diff.
export interface Chip {
  icon: IconName;
  text: string; // on the chip, short
  title: string; // in full, on hover
  kind: "passage" | "lines" | "finding";
}

// Passage is text picked out of what's asked about, as it reads: with a
// little of what's either side, to find it by again, and the section it's
// in (its heading's anchor and words), when it's in one.
export interface Passage {
  exact: string;
  prefix?: string;
  suffix?: string;
  section?: { anchor: string; title: string };
}

// readsAs is text as it reads, on one line.
export const readsAs = (s: string) => s.replace(/\s+/g, " ").trim();

// clip is text to n characters at most, … where it's cut.
export const clip = (s: string, n: number) => (s.length > n ? `${s.slice(0, n - 1).trimEnd()}…` : s);

// A passage is picked out once it's a few characters long: a stray click
// or a double-clicked letter isn't one. It's kept to what's sent.
export const passageMin = 3;
export const passageMax = 4000;
const around = 32;

// passageOf is a passage from what's selected (its text) and what reads
// before and after it, or undefined for too little.
export function passageOf(text: string, before: string, after: string, section?: Passage["section"]): Passage | undefined {
  const exact = clip(readsAs(text), passageMax);
  if (exact.length < passageMin) return undefined;
  const prefix = readsAs(before).slice(-around);
  const suffix = readsAs(after).slice(0, around);
  return { exact, ...(prefix ? { prefix } : {}), ...(suffix ? { suffix } : {}), ...(section ? { section } : {}) };
}

// AskAbout is what's asked about: what it's called, how an ask of it
// starts (with H in hand, if something's picked out of it: a passage of a
// document, lines of a pull request), and what's in hand as its chip
// shows it, with what to ask it and what dropping it asks about instead.
export interface AskAbout<H> {
  title: string;
  start: (body: string, held?: H) => Promise<TalkDetail>;
  chip: (held: H) => Chip & { placeholder: string; drop: string };
}

// aboutDoc is a document as it's asked about, by its ref, with a passage
// of it in hand (its quote, and the heading of the section it's in).
export const aboutDoc = (doc: string, title: string): AskAbout<Passage> => ({
  title,
  start: (body, p) => startAsk({ doc, body, ...(p ? { quote: { exact: p.exact, prefix: p.prefix, suffix: p.suffix }, section: p.section?.title } : {}) }),
  chip: (p) => ({ ...passageChip(p.exact), placeholder: "Ask about this passage…", drop: "Ask about the whole document" }),
});

// linesOf is lines of a file as a chip says them: 12, or 12–14.
export const linesOf = (line: number, endLine?: number) => (endLine && endLine > line ? `${line}–${endLine}` : `${line}`);

// passageChip is a passage, quoted, as a chip shows it.
export const passageChip = (exact: string, n = 40): Chip => ({ icon: "quote", text: `“${clip(readsAs(exact), n)}”`, title: readsAs(exact), kind: "passage" });

// linesChip is lines of a file in a pull request's diff, as a chip shows
// them: the file's name and the lines, its path in full on hover.
export function linesChip(path: string, line: number, endLine?: number, side?: "new" | "old"): Chip {
  const at = `${path.split("/").pop()}:${linesOf(line, endLine)}`;
  return { icon: "code", text: at, title: `${path}:${linesOf(line, endLine)}${side === "old" ? " (lines it removes)" : ""}`, kind: "lines" };
}

// anchorChip is what an ask was asked of, as its card's head shows it: a
// passage it was asked of, or lines. undefined when it was of the whole.
export function anchorChip(a: TalkDetail["anchor"]): Chip | undefined {
  if (a?.quote?.exact) return passageChip(a.quote.exact, 36);
  if (a?.path && a.line) return linesChip(a.path, a.line, a.endLine, a.side);
  return undefined;
}

// inCard is whether the card draws a proposal, to confirm there: a review
// comment, which an ask about a pull request proposes before it's kept.
// Anything else is confirmed in Conversations.
export const inCard = (p: TalkProposal) => p.kind === "comment";

// placed is an ask's proposals by the reply they came with: the first of
// the agent's turns since each was made, by its n. after are those made
// since the agent last replied (the run going now). (Times are compared
// as times: Go drops trailing zeros from fractions of a second, so their
// strings don't sort.)
export function placed(t: TalkDetail, which: (p: TalkProposal) => boolean = () => true): { under: Map<number, TalkProposal[]>; after: TalkProposal[] } {
  const ms = (iso: string) => new Date(iso).getTime();
  const replies = t.turns.filter((x) => x.by !== "you");
  const under = new Map<number, TalkProposal[]>();
  const after: TalkProposal[] = [];
  for (const p of (t.proposals ?? []).filter(which)) {
    const reply = replies.find((r) => ms(p.at) <= ms(r.at));
    if (!reply) after.push(p);
    else under.set(reply.n, [...(under.get(reply.n) ?? []), p]);
  }
  return { under, after };
}

// waiting is whether an ask is still to be answered: its agent at it, or
// your question not yet taken up (a card settled quietly isn't one). The
// card follows it until it's answered.
export function waiting(t: TalkDetail | undefined): boolean {
  if (!t || t.status !== "open") return false;
  return !!t.answering || spoken(t.turns)?.by === "you";
}

// citedSection is the section an answer's citation leads to
// ([§ How it lands](#how-it-lands)), of those the document has: by its
// anchor, or else by the words the agent gave it (#How it lands), or a
// heading named nearly so (an anchor numbered -1, or cut short). undefined
// when it isn't one of them.
export function citedSection(href: string, sections: { anchor: string; text: string }[]): string | undefined {
  if (!href.startsWith("#")) return undefined;
  let id = href.slice(1);
  try {
    id = decodeURIComponent(id);
  } catch {
    // as it is
  }
  id = id.trim();
  if (!id) return undefined;
  const named = slugger()(id);
  const anchors = sections.map((s) => s.anchor);
  const exact = anchors.find((a) => a === id.toLowerCase() || a === named);
  if (exact) return exact;
  const byWords = sections.find((s) => slugger()(s.text) === named);
  if (byWords) return byWords.anchor;
  const bare = (a: string) => a.replace(/-\d+$/, "");
  return anchors.find((a) => bare(a) === bare(named)) ?? anchors.find((a) => named.length >= 4 && (a.startsWith(named) || named.startsWith(a)));
}
