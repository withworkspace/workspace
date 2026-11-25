import { describe, expect, it } from "vitest";
import type { ModelInfo } from "../api";
import samples from "./modelSamples.json";
import { find, pick, rowOf, shortOf, treeOf } from "./modelTree";

const cursor = samples.cursor as ModelInfo[];
const acp = samples.acp as ModelInfo[];
const tree = treeOf(cursor);
const none = { effort: "", thinking: false, fast: false };

describe("treeOf", () => {
  it("puts each model's names in one row, under its maker", () => {
    expect(tree[0].vendor).toBe("");
    expect(tree[0].rows.map((r) => r.full)).toEqual(["Auto"]);
    const claude = tree.find((s) => s.vendor === "claude")!;
    const opus = claude.rows.filter((r) => r.models[0].family === "opus").map((r) => r.label);
    expect(opus.slice(0, 4)).toEqual(["Opus 5.5", "Opus 5", "Opus 4.8", "Opus 4.7"]);
    const o55 = rowOf(tree, "claude-opus-5-5-high-fast")!;
    expect(o55.label).toBe("Opus 5.5");
    expect(o55.efforts).toEqual(["low", "medium", "high", "xhigh", "max"]);
    expect(o55.fast).toEqual([false, true]);
    expect(o55.thinking).toEqual([false]);
    expect(o55.models).toHaveLength(10);
  });

  it("keeps a model with no choices as a row of its own", () => {
    const flat = treeOf([{ id: "opus", name: "Opus", about: "the latest Opus" }, { id: "sonnet", name: "Sonnet" }]);
    expect(flat).toHaveLength(1);
    expect(flat[0].rows.map((r) => [r.label, r.efforts, r.fast])).toEqual([
      ["Opus", [""], [false]],
      ["Sonnet", [""], [false]],
    ]);
  });

  it("puts Claude Code's alias and its efforts in one row", () => {
    const at = (id: string, effort?: string) => ({ id: effort ? `${id}[${effort}]` : id, name: effort ? `Haiku, ${effort}` : "Haiku", label: "Haiku", effort });
    const t = treeOf([at("haiku"), at("haiku", "low"), at("haiku", "high")]);
    expect(t).toHaveLength(1);
    const haiku = rowOf(t, "haiku[high]")!;
    expect(t[0].rows).toEqual([haiku]);
    expect(haiku.efforts).toEqual(["", "low", "high"]);
    expect(pick(haiku, { ...none, effort: "high" }).id).toBe("haiku[high]");
    expect(shortOf(haiku, haiku.models[2])).toBe("Haiku · high");
  });

  it("has ACP's models without effort or fast", () => {
    const t = treeOf(acp);
    const o55 = rowOf(t, "claude-opus-5-5")!;
    expect(o55.models.map((m) => m.id)).toEqual(["claude-opus-5-5"]);
  });
});

describe("pick", () => {
  const o5 = rowOf(tree, "claude-opus-5-high")!;

  it("finds the name for what's chosen", () => {
    expect(pick(rowOf(tree, "claude-opus-5-5-low")!, { effort: "high", thinking: false, fast: true }).id).toBe("claude-opus-5-5-high-fast");
  });

  it("goes to the nearest the row has, keeping the choice just made", () => {
    // Opus 5 has xhigh only with thinking: choosing xhigh turns thinking on.
    expect(pick(o5, { effort: "xhigh", thinking: false, fast: false }, ["effort"]).id).toBe("claude-opus-5-thinking-xhigh");
    // What's said counts over what's only wanted, the last said most.
    expect(pick(o5, { effort: "xhigh", thinking: false, fast: true }, ["fast", "effort"]).id).toBe("claude-opus-5-thinking-xhigh-fast");
    expect(pick(o5, { effort: "xhigh", thinking: false, fast: true }, ["thinking", "effort"]).id).toBe("claude-opus-5-high-fast");
    // A model with no fast stays as it is.
    expect(pick(rowOf(tree, "claude-sonnet-5-5-high")!, { effort: "high", thinking: false, fast: true }).id).toBe("claude-sonnet-5-5-high");
    // An effort it hasn't: the nearest.
    expect(pick(rowOf(tree, "glm-5.2-high")!, { ...none, effort: "medium" }).id).toBe("glm-5.2-high");
  });
});

describe("find", () => {
  it("keeps rows with every word, and takes choice words as choices", () => {
    const got = find(tree, "opus 5.5 high fast", none);
    expect(got.tree.flatMap((s) => s.rows.map((r) => r.full))).toEqual(["Claude Opus 5.5"]);
    expect(got.choice).toEqual({ effort: "high", thinking: false, fast: true });
    expect(got.said).toEqual(["fast", "effort"]);
  });

  it("finds by maker and by name", () => {
    expect(find(tree, "gpt sol", none).tree.flatMap((s) => s.rows.map((r) => r.label))).toEqual(["GPT-5.6 Sol"]);
    expect(find(tree, "codex-fast", none).tree.flatMap((s) => s.rows.map((r) => r.full))).toEqual(["Codex 5.3"]);
  });
});

it("says a name by its model and what's chosen", () => {
  const r = rowOf(tree, "claude-opus-5-5-high-fast")!;
  expect(shortOf(r, r.models.find((m) => m.id === "claude-opus-5-5-high-fast")!)).toBe("Claude Opus 5.5 · high · fast");
});
