// Fuzzy matching, as a finder wants it: the letters of what you type, in
// order, anywhere in the text; better when they sit together, start words
// or path parts, and fall in a path's last part (its file name). Space
// separates terms that must all match.
//
// The server has the same in Go (internal/find/match.go), for what Find
// asks of it. Both are tested against internal/find/testdata/fuzzy.json,
// so a change to one that isn't made to the other shows.

export interface Match {
  score: number; // higher is better
  at: number[]; // where the letters matched, to mark them
}

const boundary = (text: string, i: number) => i === 0 || "/._- ".includes(text[i - 1]) || (/[a-z]/.test(text[i - 1]) && /[A-Z]/.test(text[i]));

// matchTerm finds term in text (lowercased in lower), trying a few starts
// and keeping the best.
function matchTerm(text: string, lower: string, term: string): Match | undefined {
  const base = text.lastIndexOf("/") + 1;
  let best: Match | undefined;
  let start = lower.indexOf(term[0]);
  for (let tries = 0; start >= 0 && tries < 12; tries++, start = lower.indexOf(term[0], start + 1)) {
    const at: number[] = [];
    let score = 0;
    let i = start;
    let run = 0;
    let ok = true;
    for (const ch of term) {
      const j = lower.indexOf(ch, i);
      if (j < 0) {
        ok = false;
        break;
      }
      run = at.length && j === at[at.length - 1] + 1 ? run + 1 : 0;
      score += 1 + run * 4 + (boundary(text, j) ? 6 : 0) + (j >= base ? 2 : 0) - Math.min(j - i, 8) * 0.3;
      at.push(j);
      i = j + 1;
    }
    // Letters far apart can score below nothing, and still match.
    if (ok && (!best || score > best.score)) best = { score, at };
  }
  return best;
}

export function fuzzy(text: string, query: string): Match | undefined {
  const terms = query.trim().toLowerCase().split(/\s+/).filter(Boolean);
  if (!terms.length) return { score: 0, at: [] };
  const lower = text.toLowerCase();
  let score = 0;
  const at: number[] = [];
  for (const term of terms) {
    const m = matchTerm(text, lower, term);
    if (!m) return undefined;
    score += m.score;
    at.push(...m.at);
  }
  // Shorter paths win ties: src/a.go over src/deep/nested/a.go.
  return { score: score - text.length * 0.02, at };
}

// Marked splits text into runs, the matched letters marked.
export function marked(text: string, at: number[]): { text: string; hit: boolean }[] {
  const hits = new Set(at);
  const out: { text: string; hit: boolean }[] = [];
  for (let i = 0; i < text.length; i++) {
    const hit = hits.has(i);
    const last = out[out.length - 1];
    if (last && last.hit === hit) last.text += text[i];
    else out.push({ text: text[i], hit });
  }
  return out;
}
