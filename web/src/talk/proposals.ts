import { docAddress, type TalkProposal, type TalkTurn } from "../api";
import { clip } from "../ui/text";

// What a proposal under a conversation's reply says of itself, worked out
// from what the agent proposed; and what a card you settled says of
// itself, in the turn ws wrote for it.

// kindName is a kind's title as it's said in a sentence: "plan" for Plan,
// "ADR" for ADR.
export const kindName = (title: string) => (/^[A-Z][a-z]/.test(title) ? title[0].toLowerCase() + title.slice(1) : title);

// docLead is what a proposed document starts with, under its title: its
// first paragraph, clipped.
export function docLead(text: string): string {
  const paras = text.replace(/\r\n/g, "\n").split(/\n\s*\n/);
  const first = paras.map((p) => p.trim()).find((p) => p && !p.startsWith("#") && !p.startsWith("---"));
  if (!first) return "";
  const line = first.split("\n").join(" ");
  return clip(line, 280);
}

// startsMission: a proposal that briefs a mission, so accepting it is the
// go-ahead that starts it. A mission's brief proposed while it's being
// briefed comes with its name, its slug, or its capsules; an edit to a
// brief already under way comes with none.
export const startsMission = (p: Pick<TalkProposal, "kind" | "title" | "slug" | "capsules" | "docks">) =>
  p.kind === "brief" && (!!p.title || !!p.capsules?.length || !!p.docks?.length || !!p.slug);

// afterSettling is where settling a mission's proposal takes you, if
// anywhere. Accepting one that starts the mission goes to the mission's
// summary, wherever it was accepted: that's where it says what's
// happening now. Otherwise it's where the conversation went, if it
// moved (path, as ws gives it: its mission was renamed). slug is the
// conversation's mission as it was.
export function afterSettling(p: TalkProposal | undefined, action: "accept" | "decline", slug: string, path?: string): string | undefined {
  if (slug && action === "accept" && p && startsMission(p)) {
    const moved = path?.match(/^\/missions\/([^/]+)\//)?.[1];
    return `/missions/${moved ?? slug}`;
  }
  return path;
}

// Settled is a turn ws wrote for you settling a card, as its marker says
// it: whether you accepted it, what it was or made ("mission
// token-cache"), where that is, if it's somewhere to go, and what else
// the agent was told (what you edited before accepting).
export interface Settled {
  accepted: boolean;
  what: string;
  to?: string;
  edits?: string;
}

// settled is a turn's marker, if ws wrote it for a settled card (its
// origin is "proposal:3"): from that proposal as it stands, or, missing
// it, the turn's own first line. undefined for a turn you typed.
export function settled(t: TalkTurn, proposals: TalkProposal[] = []): Settled | undefined {
  const n = /^proposal:(\d+)$/.exec(t.origin ?? "")?.[1];
  if (!n) return undefined;
  const [first, ...rest] = t.body.trim().split(/\n\s*\n/);
  const edits = rest.join("\n\n").trim() || undefined;
  const p = proposals.find((x) => x.n === Number(n));
  const accepted = p ? p.state === "accepted" : !first.startsWith("Declined");
  if (!p) return { accepted, what: first.replace(/^(Accepted|Declined) /, "").replace(/\.$/, ""), edits };
  const quoted = (s?: string) => (s ? `“${s}”` : "");
  const made = accepted ? p.result : undefined;
  switch (p.kind) {
    case "mission":
      return made ? { accepted, what: `mission ${made}`, to: `/missions/${encodeURIComponent(made)}`, edits } : { accepted, what: `mission ${quoted(p.title)}`.trim(), edits };
    case "decision":
      return { accepted, what: `decision ${quoted(p.what?.replace(/\.$/, ""))}`.trim(), edits };
    case "document":
      if (p.to) return { accepted, what: `edit to ${made ?? p.to}`, to: made ? docAddress(made) : undefined, edits };
      return made ? { accepted, what: `document ${made}`, to: docAddress(made), edits } : { accepted, what: `document ${quoted(p.title)}`.trim(), edits };
    case "brief":
      return { accepted, what: p.plan ? "the brief and its plan" : "the edit to the brief", edits };
    case "plan":
      return { accepted, what: p.was ? "the edit to the plan" : "the plan", edits };
    case "dispatch":
      return { accepted, what: made && made !== "queued" ? `crew ${made}` : "crew", edits };
    case "replan":
      // The planner's conversation is the mission's: the card above leads there.
      return made ? { accepted, what: `replan, asked of the planner in ${made}`, edits } : { accepted, what: `replan ${quoted(clip(p.text?.replace(/\.$/, "") ?? "", 80))}`.trim(), edits };
    case "comment":
      return { accepted, what: `review comment on ${p.path}:${p.line}`, edits };
    case "plugin":
      return { accepted, what: made ? `${p.label ?? p.plugin}: ${made}` : `${p.label ?? p.plugin} ${quoted(p.title)}`.trim(), edits };
    case "artifact": {
      const v = addedVersion(made);
      const to = v ? `/artifacts/${v.id}/v${v.n}` : undefined;
      if (p.to) return { accepted, what: v ? `v${v.n} of ${p.to}` : `a version of ${p.to}, on v${p.on}`, to, edits };
      return { accepted, what: `artifact ${quoted(p.title)}`.trim(), to, edits };
    }
  }
  return { accepted, what: `proposal ${n}`, edits };
}

// spoken is the latest turn that says whose turn it is: the latest but
// for quiet ones (a card settled that needs nothing of the agent), as ws
// has it (conversation.spokenAt).
export function spoken(turns: TalkTurn[]): TalkTurn | undefined {
  let i = turns.length - 1;
  while (i > 0 && turns[i].quiet) i--;
  return turns[i];
}

// --- An artifact proposed

// An artifact as a proposed version of it needs to know it: its latest
// version, as the workspace has it now.
interface Latest {
  latest?: { n: number };
}

// A proposed artifact, or version of one: the artifact it's a version of
// (to), the version it's based on (on), and the artifact's latest version
// as it was proposed.
interface Proposed {
  to?: string;
  on?: number;
  latest?: number;
}

// artifactNext is the number a proposed version would be added as, now:
// one past the artifact's latest (1 for a new artifact). The number's
// taken when it's added.
export function artifactNext(p: Proposed, a?: Latest): number {
  if (!p.to) return 1;
  return Math.max(a?.latest?.n ?? 0, p.latest ?? 0) + 1;
}

// staleWords says a proposed version is out of date, if versions have
// been added to its artifact since it was proposed: "Proposed on v1; v2
// has been added since". Accepting still adds it, based on the version it
// was proposed on. "" when it isn't.
export function staleWords(p: Proposed, a?: Latest): string {
  const then = p.latest ?? 0;
  const now = a?.latest?.n ?? 0;
  if (!p.to || !then || now <= then) return "";
  const added: string[] = [];
  for (let n = then + 1; n <= now; n++) added.push(`v${n}`);
  const list = added.length === 1 ? added[0] : `${added.slice(0, -1).join(", ")} and ${added[added.length - 1]}`;
  return `Proposed on v${p.on ?? then}; ${list} ${added.length === 1 ? "has" : "have"} been added since`;
}

// artifactWhere says where a proposed artifact, or version, would go: a
// new one where the workspace keeps new ones (its path: .ws/artifacts/…,
// or records:artifacts/…), a version where its artifact is (its place).
// On this machine it's a file; in a repo, a commit landed there.
export function artifactWhere(p: Proposed & { path?: string }, a?: Latest & { place?: string }): string {
  const kept = (place: string | undefined) => (!place ? "" : place === ".ws" ? ", on this machine" : `, in a commit on ${place}`);
  if (p.to) return `Adds v${artifactNext(p, a)} to ${p.to}${kept(a?.place)}`;
  if (!p.path) return "";
  return `Adds ${p.path}${kept(p.path.startsWith(".ws/") ? ".ws" : p.path.split(":")[0])}`;
}

// addedVersion reads what accepting a proposed artifact made: a3.v1.
export function addedVersion(result: string | undefined): { id: string; n: number } | undefined {
  const m = /^(a[1-9]\d*)\.v([1-9]\d*)$/.exec(result ?? "");
  return m ? { id: m[1], n: Number(m[2]) } : undefined;
}

// --- Beside a conversation: what's come of it (Talk.tsx's TalkSide)

// Made is something a conversation's proposal made, or would once it's
// confirmed (open): what to call it, a short tag after that (a2.v1, d3,
// c2), and where it is now, if it's somewhere to go.
export interface Made {
  n: number;
  kind: TalkProposal["kind"];
  name: string;
  tag?: string;
  to?: string;
  open: boolean;
}

// made lists what a conversation's proposals made and what's waiting to
// be confirmed, in the order proposed. What was declined or superseded
// isn't there; of the edits to a brief or a plan accepted, only the
// latest (they're one brief, one plan). slug is the mission the
// conversation's about, if it's a mission's; titleOf is an artifact's
// title, by its id.
export function made(proposals: TalkProposal[], slug = "", titleOf: (id: string) => string | undefined = () => undefined): Made[] {
  const first = (s: string | undefined) => (s ?? "").trim().split("\n")[0].replace(/\.$/, "");
  const file = (ref: string | undefined) => (ref ?? "").slice((ref ?? "").lastIndexOf("/") + 1).replace(/^.*:/, "");
  const mission = slug ? `/missions/${encodeURIComponent(slug)}` : undefined;
  const latest = (kind: TalkProposal["kind"]) => proposals.filter((p) => p.kind === kind && p.state === "accepted").pop()?.n;
  const out: Made[] = [];
  for (const p of proposals) {
    if (p.state !== "open" && p.state !== "accepted") continue;
    const open = p.state === "open";
    const result = open ? undefined : p.result;
    const item = (name: string, more: Partial<Made> = {}) => out.push({ n: p.n, kind: p.kind, name, open, ...more });
    switch (p.kind) {
      case "artifact": {
        const v = addedVersion(result);
        item(p.title || (p.to && titleOf(p.to)) || p.to || "Artifact", { tag: v ? result : p.to ? `a version of ${p.to}` : undefined, to: v ? `/artifacts/${v.id}/v${v.n}` : undefined });
        break;
      }
      case "document":
        item(p.title || file(result || p.to) || "Document", { tag: p.to ? "edit" : undefined, to: result ? docAddress(result) : undefined });
        break;
      case "mission":
        item(p.title || "Mission", { tag: open || result ? undefined : "idea", to: result ? `/missions/${encodeURIComponent(result)}` : undefined });
        break;
      case "decision": {
        const [on, ref] = (result ?? "").split(":");
        item(clip(first(p.what), 90) || "Decision", { tag: ref, to: on ? `/missions/${encodeURIComponent(on)}` : undefined });
        break;
      }
      case "brief":
      case "plan":
        if (!open && p.n !== latest(p.kind)) break;
        item(p.kind === "plan" ? "The plan" : p.plan ? "The brief and its plan" : "The brief", { to: open || !mission ? undefined : `${mission}/brief` });
        break;
      case "dispatch":
        item(clip(first(p.text), 90) || "Crew", { tag: result, to: open ? undefined : mission });
        break;
      case "replan":
        item(clip(first(p.text), 90) || "Replan", { tag: result, to: result && mission ? `${mission}/conversations/${encodeURIComponent(result)}` : undefined });
        break;
      case "plugin":
        item(p.title || p.label || "Proposal", { tag: result || (p.items && p.items.length > 1 ? `${p.items.length} items` : undefined) });
        break;
      case "comment": {
        const [repo, number] = (p.pr ?? "").split("#");
        item(`${file(p.path)}:${p.line}`, { tag: p.pr, to: open || !number ? undefined : `/reviews/${repo}/${number}` });
        break;
      }
    }
  }
  return out;
}

// --- A revision

// revisedText is the words a proposal is mostly made of, to say what a
// revision changed of it: its brief, plan, text or decision. "" for one
// whose change is better seen another way (an artifact runs).
export function revisedText(p: TalkProposal): string {
  switch (p.kind) {
    case "decision":
      return [p.what, p.why].filter(Boolean).join("\n\n");
    case "mission":
      return [p.title ? `# ${p.title}` : "", p.brief ?? ""].filter(Boolean).join("\n\n");
    case "brief":
      return [p.brief ?? "", p.plan ?? ""].filter(Boolean).join("\n\n");
    case "plan":
      return p.plan ?? "";
    case "dispatch":
    case "replan":
    case "comment":
    case "document":
      return p.text ?? "";
    case "artifact":
      return p.note ?? "";
    case "plugin":
      return (p.items ?? [])
        .map((it, i) => [`${i + 1}. ${it.title}${it.on ? ` (${it.on})` : ""}`, ...(it.rows ?? []).map((r) => `   ${r.label}: ${r.value}`), it.markdown ? `   ${it.markdown.trim().split("\n").join("\n   ")}` : ""].filter(Boolean).join("\n"))
        .join("\n\n");
  }
  return "";
}
