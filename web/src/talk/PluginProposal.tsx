import { type ReactNode, useState } from "react";
import type { TalkProposal as Proposal } from "../api";
import { pluginIcon } from "../plugins/pluginIcon";
import { PluginItem } from "./PluginItem";
import { PluginOutcome } from "./PluginOutcome";
import { TalkPluginItems } from "./TalkPluginItems";
import { TalkProposal } from "./TalkProposal";
import { TalkProposalChoice } from "./TalkProposalChoice";
import { TalkProposalFrom } from "./TalkProposalFrom";

// Something of a plugin's a conversation's agent proposed (issues to
// create), under its reply: what the plugin says it'll do, item by item,
// each a title, the thing it changes, rows (what was, for a change) and
// what it says at length. ws doesn't read what's kept for each item, so
// there's nothing to edit here: accept all of it, or the items you tick,
// or decline it and say why, and the agent revises it. An item the plugin
// couldn't do says why, and stays to accept again; what was done stands.

export function PluginProposalCard({
  p,
  changes,
  replying,
  anchor,
  onSettle,
}: {
  p: Proposal;
  changes?: ReactNode;
  replying?: boolean;
  anchor?: string;
  onSettle: (n: number, action: "accept" | "decline", edits?: object) => Promise<void>;
}) {
  const items = p.items ?? [];
  const open = items.map((it, i) => (it.state ? 0 : i + 1)).filter(Boolean);
  const [picked, setPicked] = useState<Set<number>>(() => new Set(open));
  const [busy, setBusy] = useState(false);
  const live = p.state === "open";
  const chosen = open.filter((n) => picked.has(n));
  const several = items.length > 1;
  const tried = items.some((it) => it.problem || it.state === "made");
  const run = (action: "accept" | "decline") => {
    if (busy) return;
    setBusy(true);
    onSettle(p.n, action, action === "accept" ? { items: chosen } : {}).finally(() => setBusy(false));
  };
  const toggle = (n: number) =>
    setPicked((was) => {
      const now = new Set(was);
      if (now.has(n)) now.delete(n);
      else now.add(n);
      return now;
    });
  const accept = !several ? (tried ? "Try again" : "Accept") : chosen.length === open.length ? (tried ? `Retry ${open.length} left` : "Accept all") : `Accept ${chosen.length} of ${open.length}`;
  const made = items.filter((it) => it.state === "made");
  return (
    <TalkProposal
      state={p.state}
      anchor={anchor}
      icon={pluginIcon(p.icon)}
      label={p.label || p.plugin || ""}
      title={p.title || undefined}
      revises={p.revises}
      changes={changes}
      foot={
        live ? (
          <TalkProposalChoice
            busy={busy ? `${p.plugin} working…` : undefined}
            decline={made.length ? "Decline the rest" : "Decline"}
            declineTitle={made.length ? "Keep what's done, decline the rest" : undefined}
            accept={accept}
            ready={chosen.length > 0}
            replying={replying}
            onAccept={() => run("accept")}
            onDecline={() => run("decline")}
          />
        ) : (
          <PluginOutcome p={p} made={made} />
        )
      }
    >
      <TalkPluginItems>
        {items.map((it, i) => (
          <PluginItem
            key={i}
            n={i + 1}
            it={it}
            items={items}
            titled={several || it.title !== p.title}
            // An item's left only beside what was done: settled otherwise,
            // the card says how.
            left={it.state === "left" && made.length > 0}
            pickable={live && several && !it.state}
            picked={picked.has(i + 1)}
            onPick={() => toggle(i + 1)}
            alone={!several}
          />
        ))}
      </TalkPluginItems>
      <TalkProposalFrom plugin={p.plugin ?? ""} proposes={p.proposes} />
    </TalkProposal>
  );
}
