import { type ReactNode, useState } from "react";
import { type PluginProposalItem, type TalkProposal } from "../api";
import { Markdown } from "../markdown/LazyMarkdown";
import { pluginIcon } from "../plugins/Plugin";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/inline";
import { Button } from "../ui/Button";

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
  p: TalkProposal;
  changes?: ReactNode;
  replying?: boolean;
  anchor?: string;
  onSettle: (
    n: number,
    action: "accept" | "decline",
    edits?: object,
  ) => Promise<void>;
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
    onSettle(p.n, action, action === "accept" ? { items: chosen } : {}).finally(
      () => setBusy(false),
    );
  };
  const toggle = (n: number) =>
    setPicked((was) => {
      const now = new Set(was);
      if (now.has(n)) now.delete(n);
      else now.add(n);
      return now;
    });
  const accept = !several
    ? tried
      ? "Try again"
      : "Accept"
    : chosen.length === open.length
      ? tried
        ? `Try the ${open.length} left`
        : "Accept all"
      : `Accept ${chosen.length} of ${open.length}`;
  const made = items.filter((it) => it.state === "made");
  return (
    <div className={`proposal k-plugin s-${p.state}`} id={anchor}>
      <div className="proposal-head">
        <Icon name={pluginIcon(p.icon)} size={14} className="proposal-icon" />
        <span className="proposal-kind">{p.label || p.plugin}</span>
        {p.title && <span className="proposal-title">{p.title}</span>}
        <span className="spacer" />
        {p.revises && (
          <a
            className="proposal-revises dim"
            href={`#proposal-${p.revises}`}
            title={`It takes the place of proposal ${p.revises}`}
          >
            Revises {p.revises}
          </a>
        )}
      </div>
      {changes}
      <div className="proposal-body">
        <ol className={`plugin-items${several ? "" : " one"}`}>
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
            />
          ))}
        </ol>
        <div className="dim proposal-from">
          Proposed through the {p.plugin} plugin ({p.plugin}:{p.proposes}),
          which does what it says here once you accept it.
        </div>
      </div>
      {live ? (
        <div className="proposal-actions">
          {busy && (
            <span className="dim busy">
              <Icon name="spinner" className="spin" /> {p.plugin} is doing it…
            </span>
          )}
          <span className="spacer" />
          <Button
            variant="ghost"
            disabled={busy}
            onClick={() => run("decline")}
            title={
              made.length
                ? "What's done stands: the rest is declined"
                : undefined
            }
          >
            {made.length ? "Decline the rest" : "Decline"}
          </Button>
          <Button
            variant="primary"
            disabled={busy || !chosen.length || replying}
            onClick={() => run("accept")}
            title={
              replying
                ? "Read its reply first: the agent's still writing it"
                : undefined
            }
          >
            {accept}
          </Button>
        </div>
      ) : (
        <div className="proposal-actions settled">
          <PluginOutcome p={p} made={made} />
        </div>
      )}
    </div>
  );
}

function PluginOutcome({
  p,
  made,
}: {
  p: TalkProposal;
  made: PluginProposalItem[];
}) {
  switch (p.state) {
    case "superseded":
      return (
        <span className="settled-line dim">
          <Icon name="arrow-down" size={13} />
          <span>
            Superseded by{" "}
            <a href={`#proposal-${p.result}`}>proposal {p.result}</a>, proposed
            after it: nothing to confirm here
          </span>
        </span>
      );
    case "withdrawn":
      return (
        <span className="settled-line dim">
          <Icon name="x" size={13} /> Withdrawn
          {p.because ? `: ${p.because}` : ""}
        </span>
      );
    case "declined":
      return (
        <span className="settled-line dim">
          <Icon name="x" size={13} /> Declined
        </span>
      );
  }
  const names = made.map((it) => it.made).filter(Boolean) as string[];
  return (
    <span className="settled-line done">
      <Icon name="check-circle" size={14} />{" "}
      {names.length ? <Inline text={`Done: ${names.join(", ")}`} /> : "Done"}
      {p.because && (
        <span className="dim"> (the rest withdrawn: {p.because})</span>
      )}
    </span>
  );
}

// named is a row's value as the user reads it: an earlier item it names
// (#1) by its title, and what that made, once it's made.
function named(value: string, items: PluginProposalItem[]): string {
  const n = /^#(\d+)$/.exec(value.trim())?.[1];
  const it = n ? items[Number(n) - 1] : undefined;
  if (!it) return value;
  return it.state === "made" && it.made
    ? `${it.made} (item ${n}, ${it.title})`
    : `item ${n}, ${it.title}`;
}

function PluginItem({
  n,
  it,
  items,
  titled,
  left,
  pickable,
  picked,
  onPick,
}: {
  n: number;
  it: PluginProposalItem;
  items: PluginProposalItem[];
  titled: boolean;
  left: boolean;
  pickable: boolean;
  picked: boolean;
  onPick: () => void;
}) {
  const long = (it.markdown ?? "").length > 400;
  return (
    <li
      className={`plugin-item${it.state === "made" || left ? ` s-${it.state}` : ""}`}
    >
      {(titled || pickable || it.on || it.state === "made" || left) && (
        <div className="plugin-item-head">
          {pickable ? (
            <label className="plugin-item-pick">
              <input
                type="checkbox"
                checked={picked}
                onChange={onPick}
                aria-label={`Accept item ${n}`}
              />
              <span className="plugin-item-n dim">{n}</span>
            </label>
          ) : (
            <span className="plugin-item-n dim">{n}</span>
          )}
          {titled && <span className="plugin-item-title">{it.title}</span>}
          {it.on && (
            <span className="plugin-item-on dim">
              on <Inline text={it.on} />
            </span>
          )}
          <span className="spacer" />
          {it.state === "made" && (
            <span className="plugin-item-made">
              <Icon name="check" size={12} />{" "}
              {it.made ? <Inline text={it.made} /> : "Done"}
            </span>
          )}
          {left && <span className="dim">Not accepted</span>}
        </div>
      )}
      {!!it.rows?.length && (
        <dl className="plugin-item-rows">
          {it.rows.map((r, i) => (
            <div key={i}>
              <dt>{r.label}</dt>
              <dd>
                {r.was !== undefined && (
                  <>
                    <del className="dim">{r.was || "none"}</del> →{" "}
                  </>
                )}
                <Inline text={named(r.value, items)} />
              </dd>
            </div>
          ))}
        </dl>
      )}
      {it.markdown?.trim() &&
        (long ? (
          <details className="plugin-item-text">
            <summary className="dim">What it says</summary>
            <div className="prose">
              <Markdown text={it.markdown} />
            </div>
          </details>
        ) : (
          <div className="plugin-item-text prose">
            <Markdown text={it.markdown} />
          </div>
        ))}
      {it.problem && !it.state && (
        <div className="plugin-item-problem">
          <Icon name="alert" size={13} /> Not done: {it.problem}
        </div>
      )}
      {it.say && it.state === "made" && (
        <div className="dim plugin-item-say">{it.say}</div>
      )}
    </li>
  );
}
