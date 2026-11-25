import type { ModelInfo } from "../api";

// The efforts a model can be at, least first; "" is the model's own.
export const efforts = ["", "none", "minimal", "low", "medium", "high", "xhigh", "max"];

// What one of a model's names chooses of it.
export type ModelChoice = { effort: string; thinking: boolean; fast: boolean };

// ModelRow is one model, whatever it's at: a picker's row. Its names
// (models) are each a choice of it; efforts, thinking and fast say which
// choices it has (thinking and fast, both ways or one).
export type ModelRow = {
  key: string;
  label: string; // in its section: "Opus 5.5"
  full: string; // "Claude Opus 5.5"
  models: ModelInfo[];
  efforts: string[];
  thinking: boolean[];
  fast: boolean[];
  default: boolean; // the agent's own default is one of its names
};

// ModelSection is a maker's models; one with no vendor is those whose
// names said nothing of where they sit ("auto"), first.
export type ModelSection = { vendor: string; label: string; rows: ModelRow[] };

// Makers as they're written, where it isn't just a capital.
const vendorLabels: Record<string, string> = { gpt: "GPT", glm: "GLM", deepseek: "DeepSeek" };

export const vendorLabel = (v: string) => vendorLabels[v] ?? v.charAt(0).toUpperCase() + v.slice(1);

export const choiceOf = (m: ModelInfo): ModelChoice => ({ effort: m.effort ?? "", thinking: !!m.thinking, fast: !!m.fast });

const rowKey = (m: ModelInfo) => (m.vendor ? `${m.vendor}/${m.family ?? ""}/${m.version ?? ""}` : m.label ? `label:${m.label}` : `id:${m.id}`);

// A version's parts as numbers, for the newest first: "5.5" over "5", "k3"
// over "k2.7".
const versionParts = (v: string) => v.split(".").map((p) => Number.parseFloat(p.replace(/^[a-z]+/i, "")) || 0);
const newer = (a: string, b: string) => {
  const x = versionParts(a);
  const y = versionParts(b);
  for (let i = 0; i < Math.max(x.length, y.length); i++) {
    const d = (y[i] ?? -1) - (x[i] ?? -1);
    if (d) return d;
  }
  return 0;
};

const distinct = <T,>(xs: T[]) => [...new Set(xs)];

// treeOf is a harness's models as a picker shows them: each maker's
// models, a model's names together in its row. Makers come in the order
// the harness gave them, and in each, a family's models together, newest
// first, families as they first came.
export function treeOf(models: ModelInfo[]): ModelSection[] {
  const rows = new Map<string, ModelInfo[]>();
  for (const m of models) {
    const k = rowKey(m);
    rows.set(k, [...(rows.get(k) ?? []), m]);
  }
  const sections = new Map<string, ModelRow[]>();
  for (const [key, ms] of rows) {
    const m = ms[0];
    const vendor = m.vendor ?? "";
    const full = m.label ?? m.name ?? m.id;
    const said = vendor ? vendorLabel(vendor) : "";
    // In its maker's section, a name goes without the maker's, unless
    // that would leave a number alone ("Opus 5.5", but "GPT-5.4").
    const rest = said && full.toLowerCase().startsWith(said.toLowerCase()) ? full.slice(said.length).replace(/^[\s-]+/, "") : "";
    const label = /^[a-z]/i.test(rest) ? rest : full;
    const row: ModelRow = {
      key,
      label,
      full,
      models: ms,
      efforts: distinct(ms.map((x) => x.effort ?? "")).sort((a, b) => efforts.indexOf(a) - efforts.indexOf(b)),
      thinking: distinct(ms.map((x) => !!x.thinking)).sort(),
      fast: distinct(ms.map((x) => !!x.fast)).sort(),
      default: ms.some((x) => x.default),
    };
    sections.set(vendor, [...(sections.get(vendor) ?? []), row]);
  }
  const out: ModelSection[] = [];
  for (const [vendor, rs] of sections) {
    const families = distinct(rs.map((r) => r.models[0].family ?? ""));
    if (vendor) rs.sort((a, b) => families.indexOf(a.models[0].family ?? "") - families.indexOf(b.models[0].family ?? "") || newer(a.models[0].version ?? "", b.models[0].version ?? ""));
    out.push({ vendor, label: vendor ? vendorLabel(vendor) : "", rows: rs });
  }
  return out.sort((a, b) => Number(!!a.vendor) - Number(!!b.vendor));
}

// How far apart two efforts are, the model's own counted as medium.
const effortAt = (e: string) => efforts.indexOf(e || "medium");

// pick is the row's name nearest what's wanted. The choices said (said,
// the last first) count most, the last most of all; then thinking and
// fast as wanted; then, of efforts, the nearest.
export function pick(row: ModelRow, want: ModelChoice, said: (keyof ModelChoice)[] = []): ModelInfo {
  let best = row.models[0];
  let score = -Infinity;
  for (const m of row.models) {
    const c = choiceOf(m);
    let s = 0;
    said.forEach((on, i) => {
      if (c[on] === want[on]) s += 1000 / 2 ** i;
    });
    s += (c.thinking === want.thinking ? 100 : 0) + (c.fast === want.fast ? 100 : 0);
    s += c.effort === want.effort ? 50 : -Math.abs(effortAt(c.effort) - effortAt(want.effort));
    if (s > score) {
      score = s;
      best = m;
    }
  }
  return best;
}

// The words typed that are choices, not names: "opus 5.5 high fast".
const choiceWords: Record<string, (c: ModelChoice) => ModelChoice> = {
  fast: (c) => ({ ...c, fast: true }),
  thinking: (c) => ({ ...c, thinking: true }),
  ...Object.fromEntries(efforts.filter(Boolean).map((e) => [e, (c: ModelChoice) => ({ ...c, effort: e })])),
};

// find is the tree with only the rows whose names have every word typed,
// and what the words that are choices choose (on top of what's chosen),
// and which they said, the last first.
export function find(tree: ModelSection[], query: string, chosen: ModelChoice): { tree: ModelSection[]; choice: ModelChoice; said: (keyof ModelChoice)[] } {
  let choice = chosen;
  const said: (keyof ModelChoice)[] = [];
  const words: string[] = [];
  for (const w of query.toLowerCase().split(/\s+/).filter(Boolean)) {
    const as = choiceWords[w];
    if (as) {
      choice = as(choice);
      const on: keyof ModelChoice = w === "fast" ? "fast" : w === "thinking" ? "thinking" : "effort";
      said.splice(0, said.length, on, ...said.filter((x) => x !== on));
    } else words.push(w);
  }
  if (!words.length) return { tree, choice, said };
  const found = tree
    .map((s) => ({
      ...s,
      rows: s.rows.filter((r) => {
        const hay = `${s.label} ${r.full} ${r.models.map((m) => m.id).join(" ")}`.toLowerCase();
        return words.every((w) => hay.includes(w));
      }),
    }))
    .filter((s) => s.rows.length);
  return { tree: found, choice, said };
}

// rowOf is the row a name is in, if it's one of the tree's.
export function rowOf(tree: ModelSection[], id: string): ModelRow | undefined {
  for (const s of tree) for (const r of s.rows) if (r.models.some((m) => m.id === id)) return r;
  return undefined;
}

// Efforts as a picker says them, short.
export const effortWords: Record<string, string> = { "": "Default", none: "None", minimal: "Min", low: "Low", medium: "Med", high: "High", xhigh: "XHigh", max: "Max" };

// shortOf is a name as the picker's button says it: the model, then what's
// chosen of it ("Claude Opus 5.5 · high · fast").
export function shortOf(row: ModelRow, m: ModelInfo): string {
  const c = choiceOf(m);
  return [row.full, row.efforts.length > 1 && c.effort ? c.effort : "", row.thinking.length > 1 && c.thinking ? "thinking" : "", row.fast.length > 1 && c.fast ? "fast" : ""].filter(Boolean).join(" · ");
}
