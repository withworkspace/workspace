// Folding a file's code away: what can be folded, and what's hidden while
// it is. A fold is by a declaration or a scope where the server's parse
// found one (GET …/nav/names, internal/nav), and by indentation where it
// didn't: an import list, a literal over several lines, a language
// there's no parser for.

// Fold is lines that fold away under the line at: those after it, to and
// with to (both from 1). The line at stays, and a closing bracket's line.
export interface Fold {
  at: number;
  to: number;
}

// indent is how far in a line starts, a tab as four, or nothing for a
// blank line.
function indent(line: string): number | undefined {
  let n = 0;
  for (const c of line) {
    if (c === " ") n++;
    else if (c === "\t") n += 4;
    else if (c === "\r") continue;
    else return n;
  }
  return undefined;
}

// indentFolds is the folds indentation alone says: a line, and those
// below it that are further in.
export function indentFolds(lines: string[]): Fold[] {
  const out: Fold[] = [];
  const open: { at: number; in: number }[] = [];
  let last = 0; // the last line that wasn't blank
  const close = (to: number) => {
    while (open.length && open[open.length - 1].in >= to) {
      const o = open.pop()!;
      if (last > o.at) out.push({ at: o.at, to: last });
    }
  };
  lines.forEach((text, i) => {
    const d = indent(text);
    if (d === undefined) return;
    close(d);
    open.push({ at: i + 1, in: d });
    last = i + 1;
  });
  close(0);
  return out.sort((a, b) => a.at - b.at);
}

const closes = /^\s*[)\]}]/;

// foldsOf is a file's folds, by where they start, one to a line: what
// the server found (each a first line and a last), and indentation's
// where it found none starting. What the server found ends on its closing
// bracket's line, which stays.
export function foldsOf(lines: string[], found: [number, number][] = []): Fold[] {
  const by = new Map<number, Fold>();
  for (const [at, end] of found) {
    if (at < 1 || end > lines.length) continue; // of another reading of the file
    const to = closes.test(lines[end - 1]) ? end - 1 : end;
    if (to > at) by.set(at, { at, to });
  }
  for (const f of indentFolds(lines)) if (!by.has(f.at)) by.set(f.at, f);
  return [...by.values()].sort((a, b) => a.at - b.at);
}

// unfolded is what's folded (the lines folds start at), less any fold
// that hides one of these lines: for a line that's gone to.
export function unfolded(folds: Fold[], folded: ReadonlySet<number>, lines: number[]): ReadonlySet<number> {
  const hiding = folds.filter((f) => folded.has(f.at) && lines.some((n) => n > f.at && n <= f.to));
  if (!hiding.length) return folded;
  const next = new Set(folded);
  for (const f of hiding) next.delete(f.at);
  return next;
}
