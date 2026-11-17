import type { Bundle, BundleCheck, BundleRan, CheckVerdict } from "../api";

// What the mission's Verify tab works out from its bundles: which one to
// show, its checks in the order your attention's needed, and each in a
// line.

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

// shownBundle is the bundle the tab shows: the one picked, else the latest
// that nothing has taken the place of.
export function shownBundle(bundles: Bundle[], picked?: string): Bundle | undefined {
  return bundles.find((b) => b.id === picked) ?? [...bundles].reverse().find((b) => !b.replacedBy) ?? bundles[bundles.length - 1];
}

// listed is the bundles as the tab lists them: those standing, newest
// first, then those something took the place of.
export function listed(bundles: Bundle[]): Bundle[] {
  const newest = [...bundles].reverse();
  return [...newest.filter((b) => !b.replacedBy), ...newest.filter((b) => b.replacedBy)];
}

// checkRows is a bundle's checks: what's left for you first, then what's
// settled, each in the step's order.
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
  return [...rows.filter((r) => r.left).sort(byNumber), ...rows.filter((r) => !r.left).sort(byNumber)];
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

// waiting is whether a bundle waits for you: a Review step looks at it,
// you've said nothing of it yet, and nothing has taken its place.
export function waiting(b: Bundle): boolean {
  return !!b.asked && !b.verdict && !b.replacedBy;
}

// standing is where a bundle stands, in a few words.
export function standing(b: Bundle): string {
  if (b.replacedBy) return `Replaced by ${b.replacedBy}`;
  if (b.verdict === "approved") return "Approved by you";
  if (b.verdict === "sent back") return "Sent back";
  if (waiting(b)) return "Waiting for you";
  if (b.stale) return "Stale";
  return tally(b)
    .map(([v, n]) => `${n} ${v}`)
    .join(", ");
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
