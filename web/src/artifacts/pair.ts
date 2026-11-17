import type { ArtifactVersion } from "../api";
import { stepVersion, versionOf } from "./shown";

// Comparing two versions, apart from how it's drawn: of one artifact,
// /artifacts/a3/compare?v=1,3, or of two,
// /artifacts/compare?a=a3.v2&b=a5.v1. The first is on the left, and is
// the source diff's old side; the second's on the right, its new.

// Side is one of the two: an artifact, and its version. A version of 0
// is one the address left out, settled once the versions are read.
export interface Side {
  id: string;
  n: number;
}

export type Pair = [Side, Side];

const refShape = /^(a[1-9]\d*)(?:\.v([1-9]\d{0,4}))?$/;

// readSide reads a version as the address names it: a3.v2, or a3 (its
// latest).
export function readSide(s: string | null | undefined): Side | undefined {
  const m = refShape.exec(s?.trim() ?? "");
  return m ? { id: m[1], n: m[2] ? Number(m[2]) : 0 } : undefined;
}

// readPair reads a comparison from its address: an artifact's (its id,
// and v=1,3; v=3 is v3 against what it's based on; none, its latest
// against that), or two's (a=a3.v2&b=a5.v1). Nothing, if it names
// nothing to compare.
export function readPair(id: string | undefined, search: URLSearchParams): Pair | undefined {
  if (id !== undefined) {
    const one = readSide(id);
    if (!one || one.n) return undefined;
    const vs = (search.get("v") ?? "").split(",").map(versionOf);
    const [a, b] = vs.length > 1 ? vs : [0, vs[0]];
    return [
      { id, n: a },
      { id, n: b },
    ];
  }
  const a = readSide(search.get("a"));
  const b = readSide(search.get("b"));
  return a && b ? [a, b] : undefined;
}

type Versions = Pick<ArtifactVersion, "n" | "basedOn">[];

// settle fills in the versions the address left out, once the artifacts'
// versions are read (versions gives them, for an id): on the right, its
// latest; on the left, of the same artifact, the version the right one is
// based on (else the one before it, else itself), or another's latest. A
// side whose versions aren't read yet stays 0.
export function settle([a, b]: Pair, versions: (id: string) => Versions | undefined): Pair {
  const latest = (id: string) => Math.max(0, ...(versions(id) ?? []).map((v) => v.n));
  const right = b.n || latest(b.id);
  let left = a.n;
  const vs = versions(a.id);
  if (!left && vs && a.id === b.id && right) left = vs.find((v) => v.n === right)?.basedOn || stepVersion(vs, right, -1);
  else if (!left) left = latest(a.id);
  return [
    { id: a.id, n: left },
    { id: b.id, n: right },
  ];
}

// compareFrom is what Compare opens on from an artifact's page, at
// version n: n on the right, beside what it's based on on the left; the
// first version on the left, beside the latest; and an artifact's only
// version beside another's latest (other), if there's another. Nothing,
// if there's nothing to compare it with.
export function compareFrom(id: string, n: number, versions: Versions, other?: Side): Pair | undefined {
  if (versions.length > 1) {
    const ns = versions.map((v) => v.n);
    if (n === Math.min(...ns)) return [{ id, n }, { id, n: Math.max(...ns) }];
    return settle([{ id, n: 0 }, { id, n }], () => versions);
  }
  return other && [other, { id, n }];
}

// sideRef is a side as a ref: a3.v2 (a3, while its version isn't known).
export const sideRef = (s: Side) => (s.n ? `${s.id}.v${s.n}` : s.id);

// compareHref is a comparison's address: an artifact's, when both sides
// are of it, else two's.
export function compareHref([a, b]: Pair): string {
  if (a.id === b.id && a.n && b.n) return `/artifacts/${a.id}/compare?v=${a.n},${b.n}`;
  if (a.id === b.id && !a.n) return `/artifacts/${a.id}/compare${b.n ? `?v=${b.n}` : ""}`;
  return `/artifacts/compare?a=${sideRef(a)}&b=${sideRef(b)}`;
}

// swapped is the pair the other way about: the right on the left.
export const swapped = ([a, b]: Pair): Pair => [b, a];

// withSide is the pair with one side (0 the left, 1 the right) changed.
export function withSide(p: Pair, i: 0 | 1, s: Side): Pair {
  return i === 0 ? [s, p[1]] : [p[0], s];
}

// stepSide is the pair with one side's version stepped to the one before
// (-1) or after (+1), as the artifact's page steps.
export function stepSide(p: Pair, i: 0 | 1, by: -1 | 1, versions: Versions): Pair {
  return withSide(p, i, { id: p[i].id, n: stepVersion(versions, p[i].n, by) });
}

// samePair is whether two pairs compare the same two versions, the same
// way about.
export const samePair = (p: Pair, q: Pair) => p[0].id === q[0].id && p[0].n === q[0].n && p[1].id === q[1].id && p[1].n === q[1].n;
