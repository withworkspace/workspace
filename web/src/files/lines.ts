// Lines of a file asked for in its address: #L12, or #L12-L20 for a run
// of them (what a line number links to, a code search hit, Find's :12).

// linesAsked is the lines a hash asks for, or none.
export function linesAsked(hash: string): { from: number; to: number } | undefined {
  const m = /^#L(\d+)(?:-L(\d+))?$/.exec(hash);
  if (!m) return undefined;
  const from = Number(m[1]);
  return { from, to: Number(m[2] ?? m[1]) };
}

// rendersAt is whether a file opens rendered (markdown), given what its
// address asks: not when it asks for a line, which only its text has.
export const rendersAt = (hash: string) => !linesAsked(hash);
