import { changes } from "../annotate/diff";
import type { MissionDetail, Stage, TalkDetail, TalkProposal, TalkTurn } from "../api";
import { placed } from "../ask/ask";

// What a mission's Brief tab (Brief/Brief.tsx) works out, apart from drawing
// it: the brief and the plan as it shows them (as they are, or as the agent
// last proposed them), what each changed since the version before, as
// edits to accept or decline one by one, and the proposal confirming them
// settles; and, for what's beside them, the plan's steps as its markdown
// has them, the heading each change is under, and what the planner
// proposed for them, by the reply it came with.

export type Phase = "briefing" | "underway";

// Doc is the brief or the plan as the page shows it: as it reads, and what
// its changes are tracked against (none: nothing to compare it with, as a
// first draft has). stale: it was proposed against a version that's since
// changed, so accepting it is refused.
export interface Doc {
  text: string;
  was?: string;
  stale?: boolean;
}

export interface Shown {
  phase: Phase;
  brief: Doc;
  plan: Doc;
  // proposal is the open one the page shows, and confirming settles; the
  // drafts of the brief proposed so far, which it's the latest of; and
  // others, the open ones it replaces.
  proposal?: TalkProposal;
  draft: number;
  others: TalkProposal[];
}

const isDoc = (p: TalkProposal) => p.kind === "brief" || p.kind === "plan";

// shown is what the page shows of a mission's brief and plan, with what its
// planner's conversation proposed. Briefing, the latest proposal is the
// draft, compared with the draft you could have read before it: the last
// proposed before you last spoke (one proposed in the same answer, as a
// brief and then the brief with its plan, you never saw). The first is
// compared with nothing, or with a brief that was written already. Drafts
// are counted by what they say: the same brief again is the same draft.
// Under way, it's an edit to the brief and plan as they are, compared with
// them.
export function shown(d: Pick<MissionDetail, "state" | "brief" | "plan" | "briefUnwritten">, t?: { proposals?: TalkProposal[]; turns?: Pick<TalkTurn, "by" | "at">[] }): Shown {
  const all = (t?.proposals ?? []).filter(isDoc);
  const open = all.filter((p) => p.state === "open");
  const p = open[open.length - 1];
  const phase: Phase = d.state === "briefing" ? "briefing" : "underway";
  const brief = d.brief ?? "";
  const plan = d.plan ?? "";
  const drafts = (upTo: TalkProposal[]) => new Set(upTo.filter((x) => x.kind === "brief" && x.brief).map((x) => x.brief)).size;
  if (!p) return { phase, brief: { text: brief }, plan: { text: plan }, draft: drafts(all), others: [] };
  const others = open.slice(0, -1);
  const draft = drafts(all.slice(0, all.indexOf(p) + 1));
  const proposedBrief = p.kind === "brief" ? (p.brief ?? brief) : brief;
  const proposedPlan = p.plan ?? plan;
  if (phase === "briefing") {
    // (Times compared as times: Go drops trailing zeros from fractions of
    // a second, so their strings don't sort.)
    const ms = (iso: string) => new Date(iso).getTime();
    const spoke = Math.max(-Infinity, ...(t?.turns ?? []).filter((x) => x.by === "you" && ms(x.at) <= ms(p.at)).map((x) => ms(x.at)));
    const before = all.filter((x) => ms(x.at) < spoke).reverse();
    const lastBrief = before.find((x) => x.kind === "brief" && x.brief)?.brief;
    const lastPlan = before.find((x) => x.plan)?.plan;
    return {
      phase,
      proposal: p,
      draft,
      others,
      brief: { text: proposedBrief, was: lastBrief ?? (d.briefUnwritten || p.kind !== "brief" ? undefined : brief) },
      plan: { text: proposedPlan, was: lastPlan ?? (plan || undefined) },
    };
  }
  // What it was proposed against: none said is an empty one (the JSON
  // leaves an empty was out).
  const briefWas = p.was ?? "";
  const planWas = (p.kind === "plan" ? p.was : p.planWas) ?? "";
  return {
    phase,
    proposal: p,
    draft,
    others,
    brief: p.kind === "brief" ? { text: proposedBrief, was: briefWas, stale: briefWas !== brief } : { text: brief },
    plan: p.plan ? { text: proposedPlan, was: planWas, stale: planWas !== plan } : { text: plan },
  };
}

// Edit is a change of a document's markdown: from, found exactly once in
// it, becomes to. id names it, to accept or decline.
export interface Edit {
  id: string;
  from: string;
  to: string;
}

const linesOf = (text: string) => text.replace(/\r\n/g, "\n").replace(/\n$/, "").split("\n");

// edits are what turn was into now, as edits of its markdown's lines: each
// changed run of lines, with as many lines either side as make it found
// once in was (and keep a line on both sides, so applying it leaves the
// lines around it as they were). Applied all together, they make now.
export function edits(was: string, now: string): Edit[] {
  const a = linesOf(was);
  const b = linesOf(now);
  const once = (s: string) => {
    const i = was.indexOf(s);
    return i >= 0 && was.indexOf(s, i + 1) < 0;
  };
  const out: Edit[] = [];
  let shift = 0; // lines now has more than was, before the change at hand
  let behind = 0; // where the edit before ended, in was
  const hunks = changes(was, now);
  hunks.forEach((h, i) => {
    const newLen = h.newEnd - h.newStart + 1;
    const oldLen = h.old.length;
    let lo = h.newStart - 1 - shift;
    let hi = lo + oldLen;
    let nlo = h.newStart - 1;
    let nhi = nlo + newLen;
    shift += newLen - oldLen;
    // The next change starts here in was: an edit can't reach into it.
    const next = i + 1 < hunks.length ? hunks[i + 1].newStart - 1 - shift : a.length;
    const placed = () => hi > lo && nhi > nlo && a.slice(lo, hi).join("\n").trim() !== "" && once(a.slice(lo, hi).join("\n"));
    while (!placed()) {
      if (lo > behind) {
        lo--;
        nlo--;
      } else if (hi < next) {
        hi++;
        nhi++;
      } else break; // it can't be told apart: shown unplaced
    }
    behind = hi;
    out.push({ id: `e${i + 1}`, from: a.slice(lo, hi).join("\n"), to: b.slice(nlo, nhi).join("\n") });
  });
  return out;
}

// applied is text with some of its edits made: each where it's found in
// text as it is (edits found nowhere, or overlapping one made, are left).
export function applied(text: string, list: Pick<Edit, "from" | "to">[]): string {
  const at = list
    .map((e) => ({ e, i: e.from ? text.indexOf(e.from) : -1 }))
    .filter((x) => x.i >= 0)
    .sort((x, y) => x.i - y.i);
  let out = "";
  let pos = 0;
  for (const { e, i } of at) {
    if (i < pos) continue;
    out += text.slice(pos, i) + e.to;
    pos = i + e.from.length;
  }
  return out + text.slice(pos);
}

// Settled is what you've said of a document's edits so far, by id.
export type Settled = Record<string, "accept" | "decline">;

// reading is a document part-way through its edits: as it reads with those
// you've accepted (base), the edits still to settle, and what it comes to
// as it stands: every edit but those you declined (confirming doesn't wait
// for you to read them all). result is that, once none are left to settle.
export function reading(was: string, list: Edit[], settled: Settled): { base: string; pending: Edit[]; comes: string; result?: string } {
  const base = applied(
    was,
    list.filter((e) => settled[e.id] === "accept"),
  );
  const pending = list.filter((e) => !settled[e.id]);
  const comes = applied(
    was,
    list.filter((e) => settled[e.id] !== "decline"),
  );
  return { base, pending, comes, result: pending.length ? undefined : base };
}

const stepKinds = ["Open Pull Request", "Work", "Verify", "Review", "Decide", "Land", "Wait", "Plan"] as const;
const stepWho: Record<(typeof stepKinds)[number], NonNullable<Stage["who"]>> = {
  Work: "crew",
  Verify: "crew",
  Review: "you",
  Decide: "you",
  Land: "you",
  "Open Pull Request": "ws",
  Wait: "outside",
  Plan: "the planner",
};

// planSteps reads a plan's steps from its markdown, enough to list them
// beside it: each step's number, kind, name and what's under it. ws reads
// a plan strictly, and says what's wrong with one when it's saved; this
// lists what it can see, so the list follows the plan as it's typed, and a
// plan its planner proposed (which ws has no steps of yet) can be listed
// at all. known is the mission's steps as ws has them: a step of the same
// number keeps where it stands, and what its checks came to while its
// name's the same.
export function planSteps(plan: string, known: Stage[] = []): Stage[] {
  const lines = plan.replace(/\r\n/g, "\n").split("\n");
  const from = lines.findIndex((l) => /^##\s+Steps\s*$/i.test(l));
  if (from < 0) return [];
  const steps: Stage[] = [];
  let at: Stage | undefined;
  for (const line of lines.slice(from + 1)) {
    if (/^##\s/.test(line)) break;
    const step = /^\d+\.\s+(.*)$/.exec(line);
    if (step) {
      const rest = step[1].trim();
      const type = stepKinds.find((k) => rest === k || rest.startsWith(`${k}:`));
      const words = type ? rest.slice(type.length).replace(/^:\s*/, "") : rest;
      const name = /^\*\*(.+?)\*\*\s*(.*)$/.exec(words);
      // What the process's words it quotes are isn't what it's for.
      const text = (name ? `${name[1]} ${name[2]}` : words).replace(/\s*"[^"]*"\s*$/, "").trim();
      const title = (name ? name[1] : type && !words ? type : words).replace(/\.$/, "");
      at = { n: steps.length + 1, title, text: text || title, state: "todo", ...(type ? { type, who: stepWho[type] } : {}) };
      steps.push(at);
      continue;
    }
    const param = /^\s+-\s+([a-z ]+?):\s*(.*)$/.exec(line);
    if (!param || !at) continue;
    const [, key, value] = param;
    if (key === "in") at.in = value.replace(/^`|`$/g, "");
    else if (key === "makes") at.makes = value;
    else if (key === "limit") at.limit = value;
    else if (key === "only if") at.onlyIf = value;
    else if (key === "check" || key === "yours") (at.checks ??= []).push({ text: value });
    else if (key === "on" && at.type === "Wait") {
      at.on = value;
      at.who = value === "you" ? "you" : "outside";
    }
  }
  return steps.map((s) => {
    const k = known.find((x) => x.n === s.n);
    if (!k) return s;
    return { ...s, state: k.state, crew: k.crew, round: k.round, skipped: k.skipped, held: k.held, checks: k.title === s.title ? (k.checks ?? s.checks) : s.checks };
  });
}

// sectionOf is the heading a place in a document's markdown is under: its
// own, for a place on a heading's line.
export function sectionOf(text: string, at: number): string | undefined {
  if (at < 0) return undefined;
  const start = text.lastIndexOf("\n", at - 1) + 1;
  const own = /^#{1,3}\s+(.*)$/.exec(text.slice(start).split("\n")[0]);
  if (own) return own[1].trim();
  const before = text.slice(0, start).split("\n");
  for (let i = before.length - 1; i >= 0; i--) {
    const h = /^#{1,3}\s+(.*)$/.exec(before[i]);
    if (h) return h[1].trim();
  }
  return undefined;
}

// editsBySection is a document's edits still to settle, by the heading
// each is under in it as it reads now (base); "" for those before its
// first. An edit that puts a heading in is under the one it starts at.
export function editsBySection(base: string, pending: Edit[]): Map<string, Edit[]> {
  const by = new Map<string, Edit[]>();
  for (const e of pending) {
    // (An edit starts with lines that were there, to be found by: the
    // place it changes is past them, but under the same heading or the
    // one it adds.)
    const section = sectionOf(base, base.indexOf(e.from)) ?? "";
    by.set(section, [...(by.get(section) ?? []), e]);
  }
  return by;
}

// draftOf is which draft of the brief a proposal is: counted by what they
// say, as shown counts them (the same brief again, with its plan, is the
// same draft).
export function draftOf(proposals: TalkProposal[], p: TalkProposal): number {
  const all = proposals.filter((x) => x.kind === "brief" && x.brief);
  return new Set(all.slice(0, all.indexOf(p) + 1).map((x) => x.brief)).size;
}

// DocLine is something the planner proposed for the brief or the plan, as
// its conversation lists it beside them: what it is, what became of it,
// and whether it's the one the documents show, to confirm.
export interface DocLine {
  p: TalkProposal;
  name: string;
  fate: string;
  shown: boolean;
}

// docLines are what a conversation proposed for the brief and plan, by
// the reply each came with (after: since its agent last replied). One
// replaced in the answer it came with (the brief, then the brief with its
// plan) was never there to read, and is left out. shown is the one the
// documents show.
export function docLines(t: Pick<TalkDetail, "turns" | "proposals">, shown?: number): { under: Map<number, DocLine[]>; after: DocLine[] } {
  const all = t.proposals ?? [];
  const { under, after } = placed(t as TalkDetail, isDoc);
  const lines = (list: TalkProposal[]) =>
    list
      .filter((p) => !(p.state === "superseded" && list.some((x) => String(x.n) === p.result)))
      .map((p): DocLine => {
        const mine = p.n === shown;
        const name = p.kind === "plan" ? (p.was ? "A revision of the plan" : "The plan") : p.was !== undefined && !p.plan ? "An edit to the brief" : `Draft ${Math.max(draftOf(all, p), 1)} of the brief${p.plan ? ", with its plan" : ""}`;
        const fate = mine ? "in the documents, to confirm" : p.state === "superseded" ? "replaced" : p.state === "open" ? "to confirm" : p.state;
        return { p, name, fate, shown: mine };
      });
  return { under: new Map([...under].map(([n, list]) => [n, lines(list)])), after: lines(after) };
}

// planAsk is what the planner's asked when the plan is to be rewritten
// from the brief (or written, with none yet). unseen is a brief the
// planner hasn't read as it is: briefing, your changes to its draft aren't
// written until you confirm, so the brief goes with the ask, for it to
// propose the two together again. Otherwise the brief is as it's written,
// and the planner reads it there.
export function planAsk(o: { planned: boolean; unseen?: string }): string {
  const what = o.planned ? "Rewrite the plan" : "Write the plan";
  if (o.unseen === undefined) return `${what} to fit the brief as it reads now.`;
  return `${what} to fit the brief as I've changed it, and propose the two together again. The brief, as it reads now:\n\n${o.unseen.trim()}`;
}
