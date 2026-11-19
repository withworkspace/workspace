import type { Headline } from "../api";

// shownHeadlines is the headlines a mission's page shows, in the order
// they came: every one of yours and of outside's, and of the agents', one:
// the first. However many agents are on it (crew at work, the planner
// answering, the orchestrator looking), one banner says it's in hand.
//
// Which that is isn't worked out here: ws does (mission.Resolve keeps the
// most pressing, for ws mission show too), so given what ws gives, this
// changes nothing. It holds the page to one banner whatever it's given.
export function shownHeadlines(hs: Headline[]): Headline[] {
  let agent = false;
  return hs.filter((h) => {
    if (h.court !== "agent") return true;
    if (agent) return false;
    agent = true;
    return true;
  });
}

// titleEnd is where a sentence's title ends: at its first colon, or -1
// when it has none before its first sentence ends. A colon or a full
// stop inside brackets is neither: "Step 3 is done (c1: fixed it. Tests
// pass). Step 4 is yours, to review: …" has no title.
function titleEnd(text: string): number {
  let depth = 0;
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if (c === "\n") return -1;
    if (c === "(") depth++;
    else if (c === ")") depth = Math.max(0, depth - 1);
    else if (depth > 0) continue;
    else if (c === ":") return i;
    else if (c === "." && text[i + 1] === " ") return -1;
  }
  return -1;
}

// splitHeadline makes a headline's sentence a title and what follows:
// "Looks finished: everything in the brief is met…". A sentence with no
// title of its own (none, or one too long to be one) is titled by the
// headline's words, when it's too long to be the title itself.
export function splitHeadline(h: Headline, words: string): [string, string | undefined] {
  const text = h.text.trim();
  if (h.kind === "resume") {
    const m = /^Resume (\S+) with: ([\s\S]+)$/.exec(text);
    if (m) return [`Suggested: resume ${m[1]}`, m[2]];
  }
  const at = titleEnd(text);
  if (at >= 3 && at <= 90 && text[at + 1] === " " && text.length > at + 2) {
    const rest = text.slice(at + 2);
    return [text.slice(0, at), /^[a-z]+\b/.test(rest) && !/^[a-z]+\d/.test(rest) ? rest[0].toUpperCase() + rest.slice(1) : rest];
  }
  if (text.length <= 100) return [text, undefined];
  return [words, text];
}
