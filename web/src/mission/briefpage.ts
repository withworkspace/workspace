import { changes } from "../annotate/diff";
import type { MissionDetail, TalkProposal, TalkTurn } from "../api";

// What the brief's own page (BriefView.tsx) works out, apart from drawing
// it: the brief and the plan as it shows them (as they are, or as the agent
// last proposed them), what each changed since the version before, as
// edits to accept or decline one by one, and the proposal confirming them
// settles.

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
