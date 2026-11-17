// What's changed in a document since a version of it, by its markdown's
// lines: each change is the new lines it covers (none, for lines only
// removed) and the old lines it replaced.

export interface Change {
  // newStart..newEnd are the changed lines now, 1-based; newEnd is
  // newStart - 1 for lines only removed (before line newStart).
  newStart: number;
  newEnd: number;
  old: string[];
}

const linesOf = (text: string) => text.replace(/\r\n/g, "\n").replace(/\n$/, "").split("\n");

// changes is a line diff (Myers'), the common start and end trimmed first:
// documents change in a few places.
export function changes(before: string, after: string): Change[] {
  const a = linesOf(before);
  const b = linesOf(after);
  let pre = 0;
  while (pre < a.length && pre < b.length && a[pre] === b[pre]) pre++;
  let post = 0;
  while (post < a.length - pre && post < b.length - pre && a[a.length - 1 - post] === b[b.length - 1 - post]) post++;
  const x = a.slice(pre, a.length - post);
  const y = b.slice(pre, b.length - post);
  const same = common(x, y); // pairs of equal lines, in order
  const out: Change[] = [];
  let i = 0;
  let j = 0;
  for (const [si, sj] of [...same, [x.length, y.length] as [number, number]]) {
    if (si > i || sj > j) out.push({ newStart: pre + j + 1, newEnd: pre + sj, old: x.slice(i, si) });
    i = si + 1;
    j = sj + 1;
  }
  return out;
}

// common is the longest common subsequence of x and y, as pairs of
// indices, by Myers' greedy algorithm.
function common(x: string[], y: string[]): [number, number][] {
  const n = x.length;
  const m = y.length;
  const max = n + m;
  if (!max) return [];
  const v = new Int32Array(2 * max + 2);
  const trace: Int32Array[] = [];
  let found = false;
  for (let d = 0; d <= max && !found; d++) {
    trace.push(v.slice());
    for (let k = -d; k <= d; k += 2) {
      let i = k === -d || (k !== d && v[max + k - 1] < v[max + k + 1]) ? v[max + k + 1] : v[max + k - 1] + 1;
      let j = i - k;
      while (i < n && j < m && x[i] === y[j]) {
        i++;
        j++;
      }
      v[max + k] = i;
      if (i >= n && j >= m) {
        found = true;
        break;
      }
    }
  }
  // Walk back through the trace for the diagonals: the equal lines.
  const pairs: [number, number][] = [];
  let i = n;
  let j = m;
  for (let d = trace.length - 1; d >= 0 && (i > 0 || j > 0); d--) {
    const vd = trace[d];
    const k = i - j;
    const down = k === -d || (k !== d && vd[max + k - 1] < vd[max + k + 1]);
    const pk = down ? k + 1 : k - 1;
    const pi = d === 0 ? 0 : vd[max + pk];
    const pj = pi - pk;
    while (i > pi && j > pj && i > 0 && j > 0) {
      i--;
      j--;
      pairs.push([i, j]);
    }
    if (d > 0) {
      i = pi;
      j = pj;
    }
  }
  return pairs.reverse();
}
