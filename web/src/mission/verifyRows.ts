import type { Bundle, BundleCheck, BundleRan, CheckVerdict } from "../api";

// What the mission's Verify tab works out from its bundles: which are
// yours to approve and which one to show, its checks in the order your
// attention's needed, and each in a line.

// CheckRow is one check of a bundle as its list shows it: one the bundle
// settled (check), or a command of its step's that ws ran (ran).
export interface CheckRow {
  key: string; // c2 for the bundle's check 2, r1 for the step's check 1 that ws ran
  n: number;
  title: string; // the check as the plan says it, or the bundle's own claim
  verdict: CheckVerdict;
  // left: it's left for you, not settled: it isn't met, or the judge
  // didn't find it so.
  left: boolean;
  looked: boolean; // you've marked it as looked at
  check?: BundleCheck;
  ran?: BundleRan;
}

// BundleGroup is where a bundle is in the tab's queue: yours to approve;
// sent back, and with its crew; approved; or no longer standing (stale:
// something took its place, or its capsule has moved on since and nothing
// waits on it).
export type BundleGroup = "todo" | "back" | "approved" | "stale";

export function bundleGroup(b: Bundle): BundleGroup {
  if (b.replacedBy) return "stale";
  if (b.verdict === "approved") return b.stale ? "stale" : "approved";
  if (b.verdict === "sent back") return "back";
  return b.stale && !b.asked ? "stale" : "todo";
}

export const groupTitles: Record<BundleGroup, string> = { todo: "To approve", back: "Sent back", approved: "Approved", stale: "Stale" };

// queued is the mission's bundles in the order they want you: to approve
// (what a step waits on first), sent back, approved, stale; each newest
// first, and no group that's empty.
export function queued(bundles: Bundle[]): { group: BundleGroup; bundles: Bundle[] }[] {
  const newest = [...bundles].reverse();
  return (["todo", "back", "approved", "stale"] as BundleGroup[])
    .map((group) => {
      const mine = newest.filter((b) => bundleGroup(b) === group);
      return { group, bundles: group === "todo" ? [...mine.filter(waiting), ...mine.filter((b) => !waiting(b))] : mine };
    })
    .filter((g) => g.bundles.length > 0);
}

// toApprove is the bundles that are yours to approve, in the queue's order.
export function toApprove(bundles: Bundle[]): Bundle[] {
  return queued(bundles).find((g) => g.group === "todo")?.bundles ?? [];
}

// shownBundle is the bundle the tab shows: the one picked, else the first
// that's yours to approve. With none picked and none to approve it shows
// none: there's nothing to do, which the tab says.
export function shownBundle(bundles: Bundle[], picked?: string): Bundle | undefined {
  return bundles.find((b) => b.id === picked) ?? toApprove(bundles)[0];
}

// checkRows is a bundle's checks: what's left for you first (those you've
// approved after those you haven't), then what's settled, each in the
// step's order.
export function checkRows(b: Bundle): CheckRow[] {
  const rows: CheckRow[] = [
    ...b.checks.map(
      (c): CheckRow => ({
        key: `c${c.n}`,
        n: c.n,
        title: c.check || c.said,
        verdict: c.verdict,
        left: c.verdict !== "met" || (!!c.judged && c.judged.result !== "met"),
        looked: !!b.looked?.includes(`c${c.n}`),
        check: c,
      }),
    ),
    ...(b.ran ?? []).map(
      (r, i): CheckRow => ({
        key: `r${r.n || i + 1}`,
        n: r.n || 0,
        title: r.check,
        verdict: r.result,
        left: r.result !== "met",
        looked: !!b.looked?.includes(`r${r.n || i + 1}`),
        ran: r,
      }),
    ),
  ];
  const byNumber = (x: CheckRow, y: CheckRow) => x.n - y.n;
  return [...rows.filter((r) => r.left && !r.looked).sort(byNumber), ...rows.filter((r) => r.left && r.looked).sort(byNumber), ...rows.filter((r) => !r.left).sort(byNumber)];
}

// stillLeft is the checks left for you that you haven't said are fine: a
// check left for you that you mark (looked) is one you've approved.
export function stillLeft(rows: CheckRow[]): CheckRow[] {
  return rows.filter((r) => r.left && !r.looked);
}

// rowKey is a check's key among the page's items: the bundle's, then its
// own.
export function rowKey(b: Bundle, r: CheckRow): string {
  return `v:${b.id}:${r.key}`;
}

export const verdictWords: Record<CheckVerdict, string> = { met: "Met", unmet: "Unmet", "can't tell": "Can't tell", yours: "Yours" };

// verdictTone is the tone a verdict is shown in.
export function verdictTone(v: CheckVerdict): "done" | "danger" | "attention" | "info" {
  return v === "met" ? "done" : v === "unmet" ? "danger" : v === "yours" ? "info" : "attention";
}

// tally is a bundle's checks counted by what they came to, the commands ws
// ran among them: [["met", 4], ["can't tell", 1]].
export function tally(b: Bundle): [CheckVerdict, number][] {
  const all: CheckVerdict[] = [...b.checks.map((c) => c.verdict), ...(b.ran ?? []).map((r) => r.result)];
  return (["met", "unmet", "can't tell", "yours"] as CheckVerdict[]).map((v): [CheckVerdict, number] => [v, all.filter((x) => x === v).length]).filter(([, n]) => n > 0);
}

function count(n: number, one: string, many = one + "s"): string {
  return `${n} ${n === 1 ? one : many}`;
}

// shown is what shows a check, in a few words: "2 images, 1 recording".
export function shown(c: BundleCheck): string {
  const kinds = (c.shows ?? []).map((e) => e.kind);
  const parts: string[] = [];
  const images = kinds.filter((k) => k === "image").length;
  const videos = kinds.filter((k) => k === "video").length;
  const texts = kinds.filter((k) => k === "text").length;
  if (images) parts.push(count(images, "image"));
  if (videos) parts.push(count(videos, "recording"));
  if (texts) parts.push(count(texts, "transcript"));
  if (!parts.length && c.lines?.length) parts.push("the lines it's done in");
  return parts.join(", ");
}

// rowLine is a check in a line, under its title: what it came to, who
// said, and what shows it.
export function rowLine(b: Bundle, r: CheckRow): string {
  if (r.ran) return `${verdictWords[r.verdict]}, ws ran it`;
  const c = r.check!;
  let line = c.verdict === "yours" ? `Yours: ${b.crew} left it for you` : `${verdictWords[c.verdict]}, ${b.crew}`;
  const what = shown(c);
  if (what) line += `: ${what}`;
  if (c.judged && c.judged.result !== c.verdict) line += `. The judge says ${c.judged.result}`;
  return line;
}

// waiting is whether a bundle waits for you: a step of yours looks at it
// (a Review step, or its own step with its checks yours to say), you've
// said nothing of it yet, and nothing has taken its place.
export function waiting(b: Bundle): boolean {
  return !!b.asked && !b.verdict && !b.replacedBy;
}

// standing is where a bundle stands, in a few words.
export function standing(b: Bundle): string {
  if (b.replacedBy) return `Replaced by ${b.replacedBy}`;
  if (b.verdict === "approved") return b.stale ? "Approved, now stale" : "Approved by you";
  if (b.verdict === "sent back") return "Sent back";
  if (waiting(b)) return "Waiting for you";
  if (b.stale) return "Stale";
  return tally(b)
    .map(([v, n]) => `${n} ${v}`)
    .join(", ");
}

// nothingLeftWords says where a mission's bundles stand when none is yours
// to approve.
export function nothingLeftWords(bundles: Bundle[]): string {
  const n = (g: BundleGroup) => bundles.filter((b) => bundleGroup(b) === g).length;
  const parts: string[] = [];
  if (n("approved")) parts.push(n("approved") === 1 ? "You've approved its one standing bundle." : `You've approved all ${n("approved")} of its standing bundles.`);
  if (n("back")) parts.push(n("back") === 1 ? "1 is back with its crew: what they hand over next will wait for you." : `${n("back")} are back with their crew: what they hand over next will wait for you.`);
  if (n("stale")) parts.push(n("stale") === 1 ? "1 was replaced or has gone stale." : `${n("stale")} were replaced or have gone stale.`);
  return parts.join(" ") || "No bundle waits for you.";
}

// allClearWords says, once the last check left for you is approved, how
// many that was, and what stands as it came out: approving the bundle
// passes it with those.
export function allClearWords(b: Bundle, rows: CheckRow[]): string {
  const n = rows.filter((r) => r.left).length;
  const not = tally(b).filter(([v]) => v !== "met");
  const k = not.reduce((sum, [, c]) => sum + c, 0);
  const approved = `You've approved ${n === 1 ? "the one check" : n === 2 ? "both checks" : `all ${n} checks`} left for you in ${b.id}.`;
  if (!k) return approved;
  return `${approved} ${not.map(([v, c]) => `${c} ${v}`).join(" and ")} ${k === 1 ? "stands as it is" : "stand as they are"}: approving passes ${b.id} with ${k === 1 ? "it" : "them"}.`;
}

// sendable is the notes that would go back with a bundle: those that say
// something, by their checks' numbers.
export function sendable(notes: Record<number, string> | undefined): { check: number; text: string }[] {
  return Object.entries(notes ?? {})
    .map(([n, text]) => ({ check: Number(n), text: text.trim() }))
    .filter((n) => n.text)
    .sort((x, y) => x.check - y.check);
}

// shortCommit is a commit as it's shown.
export function shortCommit(sha?: string): string {
  return (sha ?? "").slice(0, 7);
}
