import { describe, expect, it } from "vitest";
import type { Repo } from "../../api";
import { repoGroups } from "./repoGroups";

const repo = (name: string, more: Partial<Repo> = {}): Repo => ({ name, label: name, capsules: [], cloned: true, ...more }) as Repo;

describe("repoGroups", () => {
  it("puts the ungrouped first, then groups in their order, hidden ones last, spaces apart", () => {
    const { groups, spaces } = repoGroups([
      repo("old", { group: "archive", hidden: true }),
      repo("web", { group: "apps", groupOrder: 2 }),
      repo("records", { documents: true }),
      repo("api", { group: "services", groupOrder: 1 }),
      repo("tools"),
      repo("site", { group: "apps", groupOrder: 2 }),
    ]);
    expect(groups.map(([g, rs]) => [g, rs.map((r) => r.name)])).toEqual([
      ["", ["tools"]],
      ["services", ["api"]],
      ["apps", ["web", "site"]],
      ["archive", ["old"]],
    ]);
    expect(spaces.map((r) => r.name)).toEqual(["records"]);
  });

  it("orders groups of the same order by name", () => {
    const { groups } = repoGroups([repo("b", { group: "zeta" }), repo("a", { group: "alpha" })]);
    expect(groups.map(([g]) => g)).toEqual(["alpha", "zeta"]);
  });
});
