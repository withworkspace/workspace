import type { PluginProposalItem } from "../api";
import { TalkPluginItem } from "./TalkPluginItem";

// named is a row's value as the user reads it: an earlier item it names
// (#1) by its title, and what that made, once it's made.
function named(value: string, items: PluginProposalItem[]): string {
  const n = /^#(\d+)$/.exec(value.trim())?.[1];
  const it = n ? items[Number(n) - 1] : undefined;
  if (!it) return value;
  return it.state === "made" && it.made ? `${it.made} (item ${n}, ${it.title})` : `item ${n}, ${it.title}`;
}

// PluginItem is an item of a plugin's proposal (n, of items), drawn as a
// TalkPluginItem: titled, unless it's the card's title; left, it wasn't
// accepted beside what was; pickable, it can be ticked to accept.
export function PluginItem({
  n,
  it,
  items,
  titled,
  left,
  pickable,
  picked,
  onPick,
  alone,
}: {
  n: number;
  it: PluginProposalItem;
  items: PluginProposalItem[];
  titled: boolean;
  left: boolean;
  pickable: boolean;
  picked: boolean;
  onPick: () => void;
  alone: boolean;
}) {
  return (
    <TalkPluginItem
      n={n}
      title={titled ? it.title : undefined}
      on={it.on}
      made={it.state === "made" ? it.made || true : undefined}
      left={left}
      pick={pickable ? { picked, onPick } : undefined}
      rows={it.rows?.map((r) => ({ label: r.label, was: r.was, value: named(r.value, items) }))}
      markdown={it.markdown}
      long={(it.markdown ?? "").length > 400}
      problem={!it.state ? it.problem : undefined}
      say={it.state === "made" ? it.say : undefined}
      alone={alone}
    />
  );
}
