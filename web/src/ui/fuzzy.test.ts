import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { fuzzy, marked } from "./fuzzy";

// The cases the server's matcher (internal/find/match.go) is held to as
// well, and filled in from: the page and the server have to rank alike.
interface Case {
  text: string;
  query: string;
  match: boolean;
  score?: number;
  at?: number[];
}
const all = JSON.parse(readFileSync(new URL("../../../internal/find/testdata/fuzzy.json", import.meta.url), "utf8")) as {
  cases: Case[];
  orders: { query: string; texts: string[] }[];
};

describe("the fuzzy matcher", () => {
  it("matches as the server's does, case by case", () => {
    expect(all.cases.length).toBeGreaterThan(20);
    for (const c of all.cases) {
      const m = fuzzy(c.text, c.query);
      expect(!!m, `${c.query} in ${c.text}`).toBe(c.match);
      if (!m || !c.match) continue;
      expect(Math.round(m.score * 1000) / 1000, `${c.query} in ${c.text}: score`).toBe(c.score);
      expect(m.at, `${c.query} in ${c.text}: at`).toEqual(c.at ?? []);
    }
  });

  it("orders as the server's does", () => {
    for (const o of all.orders) {
      const got = [...o.texts].sort((a, b) => fuzzy(b, o.query)!.score - fuzzy(a, o.query)!.score);
      expect(got, o.query).toEqual(o.texts);
    }
  });

  it("marks the letters that matched", () => {
    const m = fuzzy("demo/main.go", "mg")!;
    expect(marked("demo/main.go", m.at)).toEqual([
      { text: "demo/", hit: false },
      { text: "m", hit: true },
      { text: "ain.", hit: false },
      { text: "g", hit: true },
      { text: "o", hit: false },
    ]);
  });
});
