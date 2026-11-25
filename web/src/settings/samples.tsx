import { useState } from "react";
import type { AddableRepo, Repo, UpdatesInfo } from "../api";
import { Option } from "../ui/Select";
import { SettingsField } from "./SettingsField";
import { SettingsPicks } from "./SettingsPicks";
import { SettingsSelect } from "./SettingsSelect";
import { SettingsSource } from "./SettingsSource";
import { SettingsStepper } from "./SettingsStepper";

// What the Settings stories draw: controls that work in the story (each
// keeps what it's set to), the three sources, and repos, GitHub's repos
// and updates as ws would answer for them.

export const none = () => {};

export const team = <SettingsSource title="From the team's ws.toml: changed there">the team's</SettingsSource>;
export const browser = <SettingsSource title="Kept in this browser, not in a file">this browser</SettingsSource>;

export function yours(over = "", brief = false) {
  return (
    <SettingsSource yours title={`From your ws.local.toml${brief && over ? `: ${over}` : ""}`} onReset={none} resetTitle="Back to the default">
      yours{!brief && over ? ` · ${over}` : ""}
    </SettingsSource>
  );
}

export function Agent({ stretch, value = "claude" }: { stretch?: boolean; value?: string }) {
  const [v, setV] = useState(value);
  return (
    <SettingsSelect stretch={stretch} value={v} onChange={setV} aria-label="Agent">
      <Option value="claude">Claude Code</Option>
      <Option value="cursor">Cursor</Option>
      <Option value="codex" title="codex isn't on this machine">
        Codex (not here)
      </Option>
    </SettingsSelect>
  );
}

export function Model({ stretch, value = "haiku" }: { stretch?: boolean; value?: string }) {
  const [v, setV] = useState(value);
  return (
    <SettingsSelect stretch={stretch} value={v} onChange={setV} aria-label="Model">
      <Option value="haiku" note="fast · its default">
        haiku
      </Option>
      <Option value="sonnet" note="balanced">
        sonnet
      </Option>
      <Option value="opus" note="the most capable">
        opus
      </Option>
      <Option value="another">Another, by name…</Option>
    </SettingsSelect>
  );
}

export function Field({ value, ...rest }: { value: string; short?: boolean; mono?: boolean; stretch?: boolean; placeholder?: string }) {
  const [v, setV] = useState(value);
  return <SettingsField value={v} label="A setting" onCommit={setV} {...rest} />;
}

export function Number({ value }: { value: number }) {
  const [n, setN] = useState(value);
  return <SettingsStepper value={n} text={String(n)} label="Rounds" onChange={setN} />;
}

export function PickList({ picks }: { picks: string[] }) {
  const [list, setList] = useState(picks);
  return <SettingsPicks picks={list} label="Also allowed" example="claude:sonnet" onAdd={(more) => setList([...list, ...more])} onDrop={(i) => setList(list.filter((_, j) => j !== i))} />;
}

const repo = (over: Partial<Repo> & Pick<Repo, "name" | "label" | "color">): Repo => ({ cloned: true, capsules: [], aliases: [], landing: "pr", ...over }) as Repo;

export const repos: Repo[] = [
  repo({ name: "api", label: "API", color: "#4a5bd0", github: "sorrel/api", aliases: ["a"], group: "product" }),
  repo({ name: "web", label: "Web", color: "#0e7f86", github: "sorrel/web", group: "product", groupYours: true, teamGroup: "apps" }),
  repo({ name: "records", label: "Records", color: "#a8650f", github: "sorrel/records", aliases: ["rec"], landing: "direct" }),
  repo({ name: "billing", label: "Billing", color: "#b8456f", github: "sorrel/billing", cloned: false, cloning: true }),
  repo({ name: "legacy", label: "Legacy", color: "#59626f", github: "sorrel/legacy", cloned: false, problem: "repository not found" }),
  repo({ name: "scratch", label: "Scratch", color: "#2f7d4f", github: "you/scratch", yours: true }),
];

export const githubRepos: AddableRepo[] = [
  { name: "api", description: "The Sorrel API: accounts, orders, and the limits on them.", pushedAt: "2026-11-20T11:02:00Z", private: true, here: "api" },
  { name: "ledger", description: "Double-entry ledger service.", pushedAt: "2026-11-19T16:40:00Z", private: true },
  { name: "design-tokens", description: "Colours, type and spacing, as tokens, for every Sorrel app and the long description that doesn't fit the row it's in.", pushedAt: "2026-11-12T09:00:00Z" },
  { name: "old-web", pushedAt: "2025-03-02T09:00:00Z", archived: true },
  { name: "rails", description: "Sorrel's fork.", pushedAt: "2026-06-30T09:00:00Z", fork: true, here: "rails-fork" },
];

export const updates = (over: Partial<UpdatesInfo> = {}): UpdatesInfo => ({
  checks: true,
  checked: "2026-11-20T09:14:00Z",
  version: "v0.15.0",
  release: true,
  latest: "v0.15.0",
  newer: false,
  page: "https://github.com/withworkspace/workspace/releases/tag/v0.15.0",
  plugins: 0,
  ...over,
});
