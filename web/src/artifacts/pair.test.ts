import { describe, expect, it } from "vitest";
import { compareFrom, compareHref, readPair, readSide, samePair, settle, stepSide, swapped, withSide, type Pair } from "./pair";

const q = (s: string) => new URLSearchParams(s);

describe("readPair", () => {
  it("reads two versions of one artifact, left then right", () => {
    expect(readPair("a3", q("v=1,3"))).toEqual([
      { id: "a3", n: 1 },
      { id: "a3", n: 3 },
    ]);
    expect(readPair("a3", q("v=v2,v1"))).toEqual([
      { id: "a3", n: 2 },
      { id: "a3", n: 1 },
    ]);
  });

  it("leaves out what the address does: one version is the right", () => {
    expect(readPair("a3", q("v=3"))).toEqual([
      { id: "a3", n: 0 },
      { id: "a3", n: 3 },
    ]);
    expect(readPair("a3", q(""))).toEqual([
      { id: "a3", n: 0 },
      { id: "a3", n: 0 },
    ]);
    expect(readPair("a3", q("v=x,2"))).toEqual([
      { id: "a3", n: 0 },
      { id: "a3", n: 2 },
    ]);
  });

  it("reads two artifacts", () => {
    expect(readPair(undefined, q("a=a3.v2&b=a5.v1"))).toEqual([
      { id: "a3", n: 2 },
      { id: "a5", n: 1 },
    ]);
    expect(readPair(undefined, q("a=a3&b=a5"))).toEqual([
      { id: "a3", n: 0 },
      { id: "a5", n: 0 },
    ]);
  });

  it("names nothing to compare", () => {
    expect(readPair("nope", q("v=1,2"))).toBeUndefined();
    expect(readPair("a3.v2", q(""))).toBeUndefined();
    expect(readPair(undefined, q("a=a3.v2"))).toBeUndefined();
    expect(readPair(undefined, q("a=a3.v2&b=w9"))).toBeUndefined();
    expect(readSide("a3.v0")).toBeUndefined();
  });
});

describe("settle", () => {
  // v3 is based on v1: a revision proposed on v1 after v2 was added.
  const versions = { a3: [{ n: 1 }, { n: 2, basedOn: 1 }, { n: 3, basedOn: 1 }], a5: [{ n: 1 }, { n: 2, basedOn: 1 }], a7: [{ n: 1 }] };
  const of = (id: string) => versions[id as keyof typeof versions];

  it("opens on the latest and the one it's based on", () => {
    expect(settle(readPair("a3", q(""))!, of)).toEqual([
      { id: "a3", n: 1 },
      { id: "a3", n: 3 },
    ]);
    expect(settle(readPair("a3", q("v=2"))!, of)).toEqual([
      { id: "a3", n: 1 },
      { id: "a3", n: 2 },
    ]);
  });

  it("puts one with one version against itself", () => {
    expect(settle(readPair("a7", q(""))!, of)).toEqual([
      { id: "a7", n: 1 },
      { id: "a7", n: 1 },
    ]);
  });

  it("takes another artifact's latest", () => {
    expect(settle(readPair(undefined, q("a=a3&b=a5.v1"))!, of)).toEqual([
      { id: "a3", n: 3 },
      { id: "a5", n: 1 },
    ]);
  });

  it("leaves what it can't know yet", () => {
    expect(settle(readPair("a3", q("v=2"))!, () => undefined)).toEqual([
      { id: "a3", n: 0 },
      { id: "a3", n: 2 },
    ]);
    expect(settle([{ id: "a3", n: 2 }, { id: "a3", n: 1 }], of)).toEqual([
      { id: "a3", n: 2 },
      { id: "a3", n: 1 },
    ]);
  });
});

describe("compareHref", () => {
  it("is an artifact's, for two of its versions", () => {
    expect(compareHref([{ id: "a3", n: 1 }, { id: "a3", n: 3 }])).toBe("/artifacts/a3/compare?v=1,3");
    expect(compareHref([{ id: "a3", n: 0 }, { id: "a3", n: 3 }])).toBe("/artifacts/a3/compare?v=3");
    expect(compareHref([{ id: "a3", n: 0 }, { id: "a3", n: 0 }])).toBe("/artifacts/a3/compare");
  });

  it("is two's, for two artifacts", () => {
    expect(compareHref([{ id: "a3", n: 2 }, { id: "a5", n: 1 }])).toBe("/artifacts/compare?a=a3.v2&b=a5.v1");
    expect(compareHref([{ id: "a3", n: 2 }, { id: "a3", n: 0 }])).toBe("/artifacts/compare?a=a3.v2&b=a3");
  });

  it("reads back as it was", () => {
    for (const p of [
      [{ id: "a3", n: 1 }, { id: "a3", n: 3 }],
      [{ id: "a3", n: 3 }, { id: "a3", n: 1 }],
      [{ id: "a3", n: 2 }, { id: "a5", n: 1 }],
    ] as Pair[]) {
      const url = new URL(compareHref(p), "http://ws");
      const id = /^\/artifacts\/(a\d+)\/compare$/.exec(url.pathname)?.[1];
      expect(readPair(id, url.searchParams)).toEqual(p);
    }
  });
});

describe("changing sides", () => {
  const p: Pair = [
    { id: "a3", n: 1 },
    { id: "a3", n: 3 },
  ];

  it("swaps them", () => {
    expect(swapped(p)).toEqual([
      { id: "a3", n: 3 },
      { id: "a3", n: 1 },
    ]);
    expect(samePair(swapped(swapped(p)), p)).toBe(true);
    expect(samePair(swapped(p), p)).toBe(false);
  });

  it("steps one side, and stops at the ends", () => {
    const vs = [{ n: 1 }, { n: 2 }, { n: 3 }];
    expect(stepSide(p, 0, 1, vs)).toEqual([
      { id: "a3", n: 2 },
      { id: "a3", n: 3 },
    ]);
    expect(stepSide(p, 1, 1, vs)).toEqual(p);
    expect(stepSide(p, 0, -1, vs)).toEqual(p);
  });

  it("puts another artifact on one side", () => {
    expect(compareHref(withSide(p, 1, { id: "a5", n: 2 }))).toBe("/artifacts/compare?a=a3.v1&b=a5.v2");
  });
});

describe("compareFrom", () => {
  const vs = [{ n: 1 }, { n: 2, basedOn: 1 }, { n: 3, basedOn: 1 }];

  it("puts a version beside what it's based on", () => {
    expect(compareFrom("a3", 3, vs)).toEqual([
      { id: "a3", n: 1 },
      { id: "a3", n: 3 },
    ]);
    expect(compareFrom("a3", 2, vs)).toEqual([
      { id: "a3", n: 1 },
      { id: "a3", n: 2 },
    ]);
  });

  it("puts the first beside the latest", () => {
    expect(compareFrom("a3", 1, vs)).toEqual([
      { id: "a3", n: 1 },
      { id: "a3", n: 3 },
    ]);
  });

  it("puts the only version beside another artifact, if there is one", () => {
    expect(compareFrom("a7", 1, [{ n: 1 }], { id: "a5", n: 2 })).toEqual([
      { id: "a5", n: 2 },
      { id: "a7", n: 1 },
    ]);
    expect(compareFrom("a7", 1, [{ n: 1 }])).toBeUndefined();
  });
});
