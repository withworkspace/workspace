import { Fragment, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate } from "react-router";
import { getTeam, type Know, type KnowKind, narrateTeam, seenTeam, summariseTeam, type TeamTopic, type StrandState, type TeamEvent, type TeamItem, useEvents, useVisible } from "../api";
import { useKeys } from "../keys";
import { prHref } from "../reviews/Reviews";
import { Icon, type IconName } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import { Inline } from "../ui/Inline";
import { useTitle } from "../ui/useTitle";
import { toast } from "../ui/toast";
import { Loading } from "../ui/Loading";
import { Empty } from "../ui/Empty";
import { humanError } from "../ui/text";
import { Callout } from "../ui/Callout";
import { Keys } from "../ui/Keys";
import { Status, type StatusInfo } from "../ui/Status";
import { Time } from "../ui/Time";
import { dayLabel } from "../ui/when";
import { useLatest } from "../load";
import { Chip } from "../ui/Chip";
import { Count } from "../ui/Count";
import { Button } from "../ui/Button";
import { PageHead } from "../ui/PageHead";
import { GroupHead } from "../ui/GroupHead";
import { Row, RowLine, RowMain, RowMeta, RowSub, RowTitle, Rows, rowLinkClass, rowStatusClass } from "../ui/Rows";
import { Segment, Segmented } from "../ui/Segmented";
import { ToggleButton } from "../ui/ToggleButton";
import { Input } from "../ui/Input";
import { useTeamMemory } from "./memory";
import { TeamSummary } from "./TeamSummary";

// Team: what the people working in the workspace's repos have been doing,
// by day, newest first. Each pull request, and each run of one person's
// commits to one repo in a day, is one row: what it's called, what the
// change does (its gist, written by an agent from the change itself),
// what's worth knowing of it if anything is, who's on it, where it stands
// now and what happened to it that day. What's worth knowing is said under
// its change, once, and n leaves only the changes that have something. A
// line marks where you'd read to when you last looked, p arranges the same
// changes by person, and a change that touches files a capsule of yours
// has changed says so.

export const teamKeys: [string, string][] = [
  ["j / k", "Next / previous"],
  ["g / G", "First / last"],
  ["⏎", "Show or hide what happened to it"],
  ["o", "Open it: an open pull request's review here, anything else on GitHub (a change in several repos lists its pull requests)"],
  ["w", "Have what it does written, or written again if that failed"],
  ["⇧W", "Have it written for everything showing that has none"],
  ["n", "Only the changes with something worth knowing, or everything"],
  ["c", "Catch me up: have what the team's been working on written, or written again"],
  ["esc", "Back to every change, from a topic's"],
  ["p", "By person, or by day"],
  ["1 / 2 / 3", "Today / the last 7 days / as far back as ws has looked"],
  ["a", "Show or hide what bots did"],
  ["/", "Filter by text, repo or person"],
  ["r", "Look again"],
];

const periods: { since: string; label: string; title: string }[] = [
  { since: "today", label: "Today", title: "Today (1)" },
  { since: "7d", label: "7 days", title: "The last 7 days (2)" },
  { since: "", label: "All", title: "As far back as ws has looked (3)" },
];

const stateInfo: Record<StrandState, StatusInfo> = {
  "in-progress": { icon: "circle-dashed", tone: "quiet", words: "In progress" },
  "in-review": { icon: "pr", tone: "waiting", words: "In review" },
  landed: { icon: "check-circle", tone: "done", words: "Landed" },
  closed: { icon: "x-circle", tone: "quiet", words: "Closed" },
};

// What each kind of thing worth knowing is called, and drawn with.
const knowInfo: Record<KnowKind, { label: string; icon: IconName; title: string }> = {
  contract: { label: "Contract", icon: "link", title: "Something other code calls or reads has changed shape" },
  action: { label: "To do", icon: "alert", title: "Something you may have to do" },
  convention: { label: "How we work", icon: "book", title: "How things are done here has changed" },
  removed: { label: "Removed", icon: "minus", title: "Something was taken away or deprecated" },
  behaviour: { label: "Behaviour", icon: "eye", title: "What users or callers see has changed" },
};

// fileHref is a file on its repo's default branch, in Files.
const fileHref = (repo: string, path: string) => `/files/${encodeURIComponent(repo)}?${new URLSearchParams({ path })}`;

// useTeamEvents calls changed when there's been a new look, or a gist's
// been written: it holds the Team topic, followed on its socket, while the page shows,
// which is also what has the server look at git and GitHub every couple
// of minutes, and write the gists of what's happened lately.
function useTeamEvents(changed: () => void) {
  useEvents("team", { changed });
}

// How the page was left (memory.ts): what was last read of each period,
// and what was picked, opened and shown.
const remember = useTeamMemory.setState;

export function TeamPage() {
  useTitle("Team");
  const navigate = useNavigate();
  const { period, showBots, onlyKnow, byPeople, foldSummary, opened } = useTeamMemory();
  const view = useTeamMemory((m) => m.views[m.period]);
  const [error, setError] = useState<string>();
  const [loading, setLoading] = useState(false);
  const [q, setQ] = useState("");
  const filter = useRef<HTMLInputElement>(null);
  // When you last looked, as it was when this visit began: null if you
  // never had. What's since is new, for as long as you stay.
  // The topic whose changes are the only ones showing, if one is.
  const [topic, setTopic] = useState<TeamTopic>();
  const [marker, setMarker] = useState<string | null>();
  const visible = useVisible();
  useEffect(() => {
    if (marker === undefined || !visible) return;
    // Looking moves it: once you've been here a moment, this is seen.
    const t = setTimeout(() => seenTeam().catch(() => {}), 4000);
    return () => clearTimeout(t);
  }, [marker, visible]);

  const begin = useLatest();
  const load = useCallback(
    (refresh = false) => {
      const fresh = begin();
      setLoading(true);
      return getTeam(period, refresh)
        .then(
          (v) => {
            remember((m) => ({ views: { ...m.views, [period]: v } }));
            if (!fresh()) return v;
            setError(undefined);
            setMarker((m) => (m === undefined ? (v.seen ?? null) : m));
            return v;
          },
          (e: Error) => {
            if (fresh()) setError(e.message);
            return undefined;
          },
        )
        .finally(() => fresh() && setLoading(false));
    },
    [period, begin],
  );
  useEffect(() => {
    load();
  }, [load]);
  useTeamEvents(load);

  const people = useMemo(() => new Map((view?.people ?? []).map((p) => [p.key, p])), [view]);
  const names = (keys: string[]) => keys.map((k) => people.get(k)?.short ?? k);
  const repoLabel = (repo: string) => view?.repos[repo]?.label ?? repo;

  // A strand's latest row in the period is where what's worth knowing of
  // it is said: its earlier rows have its gist alone.
  const latest = useMemo(() => new Set((view?.know ?? []).map((k) => k.item)), [view]);
  const worth = useMemo(() => new Set((view?.know ?? []).map((k) => k.strand)).size, [view]);
  // What the filter leaves: rows whose name, gist, repo, people or number
  // has every word. Bots' are left out unless they're shown, or looked for.
  // With only what's worth knowing asked for, it's the changes that have
  // something, each once.
  const days = useMemo(() => {
    const words = q.toLowerCase().split(/\s+/).filter(Boolean);
    const has = (it: TeamItem) => {
      const who = it.people.map((k) => `${people.get(k)?.name ?? ""} ${people.get(k)?.login ?? ""}`).join(" ");
      const know = (it.know ?? []).map((k) => `${k.text} ${k.file} ${knowInfo[k.kind]?.label ?? ""}`).join(" ");
      const repos = (it.repos ?? [it.repo]).map((r) => `${r} ${view?.repos[r]?.label ?? ""}`).join(" ");
      const prs = (it.prs ?? []).map((p) => `#${p.number} ${p.repo}#${p.number}`).join(" ");
      const touches = (it.touches ?? []).map((t) => `${t.title} ${t.files.join(" ")}`).join(" ");
      const hay = `${it.name} ${it.gist ?? ""} ${know} ${touches} ${repos} ${prs} ${who} ${it.pr ? `#${it.pr} ${it.repo}#${it.pr}` : ""} ${it.words}`.toLowerCase();
      return words.every((w) => hay.includes(w));
    };
    return (view?.days ?? [])
      .map((d) => {
        const matching = d.items.filter((it) => (!topic || topic.strands.includes(it.strand)) && (!onlyKnow || latest.has(it.key)) && (!words.length || has(it)));
        const bots = matching.filter((it) => it.bot).length;
        return { date: d.date, bots, items: matching.filter((it) => !it.bot || showBots || words.length > 0) };
      })
      .filter((d) => d.items.length || d.bots);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [view, q, people, latest, topic, showBots, onlyKnow]);
  const order = days.flatMap((d) => d.items);
  const isNew = (it: TeamItem) => !!marker && it.at > marker;
  // What's new to you: the changes with something since you last looked.
  const fresh = new Set(order.filter(isNew).map((it) => it.strand)).size;
  // The line under what's new: before the first row you'd already seen.
  const seenFrom = fresh > 0 ? order.find((it) => !isNew(it))?.key : undefined;

  // By person: each change once, at its latest row, under whose it is and
  // under whoever else was on it. People come in the order they last did
  // something; nobody's listed for having done nothing.
  const groups = useMemo(() => {
    const first = new Map<string, TeamItem>();
    const on = new Map<string, Set<string>>();
    for (const it of days.flatMap((d) => d.items)) {
      if (!first.has(it.strand)) first.set(it.strand, it);
      const all = on.get(it.strand) ?? new Set<string>();
      it.people.forEach((k) => all.add(k));
      on.set(it.strand, all);
    }
    const out = new Map<string, { theirs: TeamItem[]; reviewing: TeamItem[] }>();
    for (const [strand, it] of first) {
      for (const key of on.get(strand) ?? []) {
        if (people.get(key)?.bot && !showBots) continue;
        const g = out.get(key) ?? { theirs: [], reviewing: [] };
        (key === it.people[0] ? g.theirs : g.reviewing).push(it);
        out.set(key, g);
      }
    }
    // What's still open before what's landed.
    const rank = (it: TeamItem) => (it.state === "in-review" || it.state === "in-progress" ? 0 : 1);
    return [...out].map(([key, g]) => ({ key, theirs: [...g.theirs].sort((a, b) => rank(a) - rank(b)), reviewing: g.reviewing }));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [days, people, showBots]);

  // The rows as they're shown, in order, each with an id of its own: by
  // person, a change is under everyone on it.
  const entries = byPeople
    ? groups.flatMap((g) => [...g.theirs, ...g.reviewing].map((it) => ({ id: `${g.key}/${it.key}`, it })))
    : order.map((it) => ({ id: it.key, it }));
  const [selected, setSelected] = useState(() => useTeamMemory.getState().selected);
  const index = Math.max(0, entries.findIndex((e) => e.id === selected));
  const current = entries[index]?.it;
  const select = (i: number) => {
    const e = entries[Math.min(Math.max(i, 0), entries.length - 1)];
    if (!e) return;
    setSelected(e.id);
    remember({ selected: e.id });
    document.getElementById(`team-row-${e.id}`)?.scrollIntoView({ block: "nearest" });
  };
  const toggle = (it?: TeamItem) => {
    if (!it) return;
    remember((m) => ({ opened: { ...m.opened, [it.key]: !m.opened[it.key] } }));
  };
  // An open pull request has a review here; anything else is on GitHub.
  const open = (it?: TeamItem) => {
    if (!it) return;
    if (it.prs?.length) {
      // One change in several repos: its pull requests are listed in it.
      remember((m) => ({ opened: { ...m.opened, [it.key]: true } }));
      toast(`It's ${it.prs.length} pull requests: each is listed under it.`);
    } else if (it.kind === "pr" && it.pr && (it.state === "in-review" || it.state === "in-progress")) navigate(prHref(it.repo, it.pr));
    else if (it.url) window.open(it.url, "_blank", "noopener");
    else toast("There's nowhere to open this yet: it has no address on GitHub.");
  };
  const pick = (since: string) => {
    setTopic(undefined);
    remember({ period: since });
  };
  const bots = () => {
    remember({ showBots: !showBots });
  };
  const know = () => {
    remember({ onlyKnow: !onlyKnow });
  };
  // The period's account: written when asked for, from what's been read.
  const summary = view?.summary;
  const catchUp = () => {
    if (!summary) {
      toast(`Nothing writes these here${narrative?.off ? `: ${narrative.off}` : ""}.`);
      return;
    }
    if (!summary.can) {
      toast("None of these changes has been read yet: there's nothing to write it from.");
      return;
    }
    remember({ foldSummary: false });
    summariseTeam(period).then(
      (r) => {
        if (!r.started) toast("It's being written.");
        load();
      },
      (e: Error) => toast(humanError(e.message), { tone: "danger" }),
    );
  };
  const arrange = (to: boolean) => {
    remember({ byPeople: to });
  };
  const refresh = () =>
    load(true).then((v) => {
      if (v?.error) toast(humanError(v.error), { tone: "danger" });
    });
  const narrative = view?.narrative;
  // Having gists written: one row's, or all that are showing and have none.
  const write = (what: { strand?: string; since?: string }) => {
    if (!narrative?.on) {
      toast(`Nothing writes these here${narrative?.off ? `: ${narrative.off}` : ""}.`);
      return;
    }
    narrateTeam(what).then(
      (r) => {
        if (!r.started) toast(what.strand ? "It's written, or being written." : "There's nothing left to write.");
        load();
      },
      (e: Error) => toast(humanError(e.message), { tone: "danger" }),
    );
  };
  const writeRow = (it?: TeamItem) => it && (it.narrative === "unwritten" || it.narrative === "failed") && write({ strand: it.strand });
  const writeAll = () => write({ since: period });
  useKeys({
    j: () => select(index + 1),
    ArrowDown: () => select(index + 1),
    k: () => select(index - 1),
    ArrowUp: () => select(index - 1),
    g: () => select(0),
    G: () => select(entries.length - 1),
    Enter: () => toggle(current),
    o: () => open(current),
    w: () => writeRow(current),
    W: writeAll,
    n: know,
    c: catchUp,
    Escape: () => setTopic(undefined),
    p: () => arrange(!byPeople),
    a: bots,
    r: refresh,
    "1": () => pick(periods[0].since),
    "2": () => pick(periods[1].since),
    "3": () => pick(periods[2].since),
    "/": () => filter.current?.focus(),
  });

  // A row, under the id it has where it's shown.
  const row = (id: string, it: TeamItem) => (
    <TeamRow
      key={id}
      id={id}
      item={it}
      names={names(it.people)}
      repoOf={(r) => ({ label: repoLabel(r), color: view?.repos[r]?.color })}
      selected={entries[index]?.id === id}
      open={!!opened[it.key]}
      fresh={isNew(it)}
      dated={byPeople}
      know={byPeople || latest.has(it.key)}
      who={(e) => people.get(e.who ?? "")?.short}
      onHover={() => entries[index]?.id !== id && setSelected(id)}
      onToggle={() => {
        remember({ selected: id });
        setSelected(id);
        toggle(it);
      }}
      onOpen={() => open(it)}
      onWrite={() => writeRow(it)}
    />
  );

  const head = (
    <PageHead
      title="Team"
      actions={
        <>
          {view && (
            <span className="dim reviews-asked" title="When git and GitHub were last looked at. While this page is open they're looked at every couple of minutes.">
              {loading ? (
                <>
                  <Spinner size={12} /> Looking…
                </>
              ) : (
                <>
                  As of <Time iso={view.at} as="relative" />
                </>
              )}
            </span>
          )}
          <Segmented value={byPeople ? "people" : "days"} onChange={(v) => arrange(v === "people")} aria-label="Arranged">
            <Segment value="days" title="By day, newest first (p)">
              Days
            </Segment>
            <Segment value="people" title="By person: what each has open, has landed, and has been reviewing (p)">
              People
            </Segment>
          </Segmented>
          <Segmented value={period} onChange={pick} aria-label="Period">
            {periods.map((p) => (
              <Segment key={p.since} value={p.since} title={p.title}>
                {p.label}
              </Segment>
            ))}
          </Segmented>
          {narrative?.on && (
            <ToggleButton variant="ghost" size="small" pressed={onlyKnow} onChange={know} title="Only the changes with something worth knowing (n)">
              <Icon name="eye" size={13} /> Worth knowing {worth > 0 && <Count tone="attention">{worth}</Count>}
            </ToggleButton>
          )}
          <Input
            ref={filter}
            className="reviews-filter"
            value={q}
            placeholder="Filter: text, repo, person"
            aria-label="Filter what happened"
            onChange={(e) => setQ(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Escape" || e.key === "Enter") {
                e.preventDefault();
                if (e.key === "Escape") setQ("");
                e.currentTarget.blur();
              }
            }}
          />
          <Button variant="ghost" size="small" onClick={refresh} disabled={loading} title="Look at git and GitHub again (r)">
            <Icon name="rotate" size={13} /> Refresh <Keys k="r" />
          </Button>
        </>
      }
    />
  );

  if (!view) {
    return (
      <div className="page">
        {head}
        {error ? (
          <Empty icon="user" title="Nothing to show">
            {humanError(error)}
          </Empty>
        ) : (
          <Loading what="Reading what's landed, and asking GitHub for pull requests…" />
        )}
      </div>
    );
  }
  const total = view.days.reduce((n, d) => n + d.items.length, 0);
  return (
    <div className="page team-page">
      {head}
      {(error || view.error) && (
        <Callout tone="attention" title="The repos couldn't be looked at just now" actions={<Button onClick={refresh}>Try again</Button>}>
          {humanError(error ?? view.error ?? "")} What's here is as of <Time iso={view.at} as="relative" />.
        </Callout>
      )}
      {!!view.problems?.length && (
        <Callout tone="quiet" icon="info" title={`${view.problems.length === 1 ? "One thing" : `${view.problems.length} things`} couldn't be read`}>
          {view.problems.slice(0, 8).map((p) => (
            <div key={p}>{p}</div>
          ))}
          {view.problems.length > 8 && <div className="dim">and {view.problems.length - 8} more</div>}
        </Callout>
      )}
      {!view.github && (
        <Callout tone="quiet" icon="info" title="This is what git knows">
          There's no GitHub to ask here, so pull requests show only once they've merged, and nobody's reviews do.
        </Callout>
      )}
      {!total && (
        <Empty icon="user" title={period === "today" ? "Nothing yet today" : "Nothing has happened"}>
          {period === "today" ? "Nothing has landed on a default branch or happened to a pull request today." : "Nothing has landed on a default branch or happened to a pull request in this time."} Press <Keys k="3" /> to look further
          back.
        </Empty>
      )}
      {total > 0 && !days.length && !!q && <p className="dim">Nothing matches “{q}”.</p>}
      {total > 0 && (fresh > 0 || (narrative && (narrative.writing > 0 || narrative.unwritten > 0)) || (summary && !summary.at && summary.can > 0)) && (
        <p className="team-narrative dim">
          {fresh > 0 && marker && (
            <span className="team-fresh">
              {fresh} new since you looked <Time iso={marker} as="relative" />
            </span>
          )}
          {!!narrative?.writing && (
            <span className="team-writing" title="An agent is reading each change and writing what it does">
              <Spinner size={12} /> Reading {narrative.writing} {narrative.writing === 1 ? "change" : "changes"}
            </span>
          )}
          {summary && !summary.at && !summary.writing && summary.can > 0 && (
            <Button variant="link" onClick={catchUp} title="An agent writes what the team's been working on in this period, from the changes that have been read (c)">
              Catch me up
            </Button>
          )}
          {narrative?.on && narrative.unwritten > 0 && (
            <span>
              {narrative.unwritten} {narrative.unwritten === 1 ? "change hasn't" : "changes haven't"} been read: those from before yesterday wait until you ask.{" "}
              <Button variant="link" onClick={writeAll} title="Have an agent read them (⇧W)">
                Read them
              </Button>
            </span>
          )}
        </p>
      )}
      {total > 0 && summary && (summary.topics?.length || summary.writing || summary.failed) && (
        <TeamSummary
          summary={summary}
          folded={foldSummary}
          onFold={() => remember({ foldSummary: !foldSummary })}
          on={topic?.name}
          onTopic={(t) => setTopic(topic?.name === t.name ? undefined : t)}
          names={names}
          onWrite={catchUp}
        />
      )}
      {topic && (
        <p className="team-narrative">
          <span>
            Only <strong>{topic.name}</strong>: {topic.strands.length} {topic.strands.length === 1 ? "change" : "changes"}.
          </span>
          <Button variant="link" onClick={() => setTopic(undefined)}>
            Show everything <Keys k="esc" />
          </Button>
        </p>
      )}
      {total > 0 && onlyKnow && !days.length && !q && (
        <Empty icon="eye" title="Nothing worth knowing">
          {narrative?.unwritten ? "Not in the changes that have been read, at least." : "None of these changes is one you'd be caught out by."} Press <Keys k="n" /> for everything.
        </Empty>
      )}
      {!byPeople &&
        days.map((d) => (
          <section key={d.date} aria-label={d.date}>
            <GroupHead count={d.items.length}>
              {dayLabel(`${d.date}T12:00:00`)}
            </GroupHead>
            <Rows className="team-rows" role="listbox" aria-label={d.date}>
              {d.items.map((it) => (
                <Fragment key={it.key}>
                  {it.key === seenFrom && marker && (
                    <li className="team-seen-line" role="separator">
                      <span>
                        You'd seen what's below <Time iso={marker} as="relative" />
                      </span>
                    </li>
                  )}
                  {row(it.key, it)}
                </Fragment>
              ))}
            </Rows>
            {d.bots > 0 && !q && (
              <Button variant="link" className="team-bots" onClick={bots} aria-expanded={showBots}>
                <Icon name={showBots ? "chevron-down" : "chevron-right"} size={12} /> {d.bots} automated
              </Button>
            )}
          </section>
        ))}
      {byPeople &&
        groups.map((g) => (
          <section key={g.key} aria-label={people.get(g.key)?.name ?? g.key}>
            <GroupHead title={people.get(g.key)?.login}>
              {people.get(g.key)?.name ?? g.key}
            </GroupHead>
            {g.theirs.length > 0 && (
              <Rows className="team-rows" role="listbox" aria-label="Theirs">
                {g.theirs.map((it) => row(`${g.key}/${it.key}`, it))}
              </Rows>
            )}
            {g.reviewing.length > 0 && (
              <>
                <h3 className="team-sub">Reviewing</h3>
                <Rows className="team-rows" role="listbox" aria-label="Reviewing">
                  {g.reviewing.map((it) => row(`${g.key}/${it.key}`, it))}
                </Rows>
              </>
            )}
          </section>
        ))}
    </div>
  );
}

function KnowChip({ kind }: { kind: KnowKind }) {
  const info = knowInfo[kind] ?? knowInfo.behaviour;
  return (
    <Chip tone="attention" className="team-know-kind" title={info.title}>
      <Icon name={info.icon} size={11} /> {info.label}
    </Chip>
  );
}

interface TeamRowProps {
  id: string;
  item: TeamItem;
  names: string[];
  repoOf: (repo: string) => { label: string; color?: string };
  selected: boolean;
  open: boolean;
  // fresh: something happened to it since you last looked.
  fresh: boolean;
  // dated: it's not under its day, so when is said with the day.
  dated: boolean;
  // know: this is the row what's worth knowing of its change is said on.
  know: boolean;
  who: (e: TeamEvent) => string | undefined;
  onHover: () => void;
  onToggle: () => void;
  onOpen: () => void;
  onWrite: () => void;
}

function TeamRow({ id, item: it, names, repoOf, selected, open, fresh, dated, know, who, onHover, onToggle, onOpen, onWrite }: TeamRowProps) {
  const status: StatusInfo = it.kind === "commits" ? { icon: "commit", tone: "done", words: "Landed" } : stateInfo[it.state];
  const here = it.kind === "pr" && (it.state === "in-review" || it.state === "in-progress");
  const repo = repoOf(it.repo);
  // One change in several repos: how many of its pull requests have landed.
  const several = it.prs && it.prs.length > 1 ? it.prs : undefined;
  const landed = several?.filter((p) => p.state === "landed").length ?? 0;
  return (
    <Row selected={selected} id={`team-row-${id}`} role="option" aria-selected={selected} className={`team-row${open ? " open" : ""}${fresh ? " fresh" : ""}`} onMouseMove={onHover}>
      <div className={rowLinkClass("team-row-link")} role="button" tabIndex={-1} aria-expanded={open} onClick={onToggle}>
        <Status s={status} iconOnly size={16} className={rowStatusClass()} />
        <RowMain className="team-row-main">
          <RowLine>
            {fresh && <span className="team-new" title="Something's happened to it since you last looked" />}
            <RowTitle>{it.name}</RowTitle>
            {several ? (
              <Chip tone="quiet" className="team-repo" title={(it.repos ?? []).map((r) => repoOf(r).label).join(", ")}>
                {(it.repos ?? []).slice(0, 4).map((r) => (
                  <span key={r} className="repo-dot" style={{ background: repoOf(r).color }} />
                ))}
                {it.repos?.length} repos
              </Chip>
            ) : (
              <Chip tone="quiet" className="team-repo" title={it.repo}>
                <span className="repo-dot" style={{ background: repo.color }} />
                {repo.label}
                {it.pr ? <span className="mono"> #{it.pr}</span> : null}
              </Chip>
            )}
            {it.touches?.map((t) => (
              <Chip tone="progress" key={t.name} className="team-touch" title={`It changes how things are done:\n${t.files.join("\n")}`}>
                {t.title}
              </Chip>
            ))}
            <span className={`team-state tone-${status.tone}`}>
              {status.words}
              {several && landed > 0 && landed < several.length ? `, ${landed} of ${several.length} landed` : ""}
            </span>
          </RowLine>
          {it.gist && <Inline text={it.gist} className="team-gist" />}
          {it.narrative === "writing" && !it.gist && (
            <span className="team-gist dim">
              <Spinner size={11} /> Reading the change…
            </span>
          )}
          {know && it.know?.map((k) => <KnowLine key={`${k.kind}:${k.file}:${k.text}`} know={k} repo={it.repo} />)}
          {it.near?.map((n) => (
            <span key={n.capsule} className="team-know-line team-near" title={n.files.join("\n")}>
              <Chip tone="progress" className="team-know-kind" title="A capsule of yours has changed some of the same files">
                <Icon name="box" size={11} /> Near you
              </Chip>
              <span>
                Also changed in your <span className="mono">{n.capsule}</span>: {n.files.slice(0, 2).map((f) => f.split("/").pop()).join(", ")}
                {n.files.length > 2 ? `, and ${n.files.length - 2} more` : ""}
              </span>
            </span>
          ))}
          <RowSub className="team-row-sub">{it.words}</RowSub>
        </RowMain>
        <RowMeta className="team-people" title={names.join(", ")}>
          {names.join(", ")}
        </RowMeta>
        {it.additions || it.deletions ? (
          <RowMeta className="pr-size">
            <span className="plus">+{it.additions ?? 0}</span> <span className="minus">−{it.deletions ?? 0}</span>
          </RowMeta>
        ) : (
          <RowMeta className="pr-size"  />
        )}
        <Time iso={it.at} as={dated ? "relative" : "clock"} className="age" />
      </div>
      {open && (
        <div className="team-events">
          {several && (
            <ul className="team-prs">
              {several.map((p) => {
                const s = stateInfo[p.state];
                const open = p.state === "in-review" || p.state === "in-progress";
                const inner = (
                  <>
                    <Status s={s} iconOnly size={13} />
                    <span className="repo-dot" style={{ background: repoOf(p.repo).color }} />
                    <span>{repoOf(p.repo).label}</span>
                    <span className="mono dim">#{p.number}</span>
                    <span className={`team-state tone-${s.tone}`}>{s.words}</span>
                  </>
                );
                return (
                  <li key={`${p.repo}#${p.number}`}>
                    {open ? (
                      <Link to={prHref(p.repo, p.number)} title="Review it here">
                        {inner}
                      </Link>
                    ) : p.url ? (
                      <a href={p.url} target="_blank" rel="noreferrer" title="Open on GitHub">
                        {inner}
                      </a>
                    ) : (
                      <span>{inner}</span>
                    )}
                  </li>
                );
              })}
            </ul>
          )}
          <ol>
            {it.events.map((e) => (
              <li key={e.id}>
                <Time iso={e.at} as="clock" className="dim num" />
                <Icon name={eventIcon(e)} size={13} />
                <span className="team-event-words">
                  {e.kind !== "closed" && <strong>{who(e) ?? e.login ?? e.name ?? "Someone"} </strong>}
                  {eventWords(e)}
                  {several && <span className="dim"> in {repoOf(e.repo).label}</span>}
                  {e.kind === "landed" && <span className="team-subject">{e.title}</span>}
                </span>
                {e.commit && <span className="mono dim">{e.commit.slice(0, 7)}</span>}
                {!!e.paths?.length && (
                  <span className="team-paths dim" title={e.paths.join("\n")}>
                    {e.paths.slice(0, 3).join(", ")}
                    {(e.files ?? e.paths.length) > 3 ? `, and ${(e.files ?? e.paths.length) - 3} more` : ""}
                  </span>
                )}
              </li>
            ))}
          </ol>
          {it.narrative === "failed" && <p className="team-gist-error">What it does couldn't be written: {humanError(it.gistError ?? "")}</p>}
          <div className="team-actions">
            {!several && (
              <Button variant="ghost" size="small" onClick={onOpen} title="Open it (o)">
                <Icon name={here ? "pr" : "external"} size={13} /> {here ? "Review it here" : "Open on GitHub"} <Keys k="o" />
              </Button>
            )}
            {(it.narrative === "unwritten" || it.narrative === "failed") && (
              <Button variant="ghost" size="small" onClick={onWrite} title="An agent reads the change and writes what it does, and what's worth knowing of it (w)">
                <Icon name="sparkle" size={13} /> {it.narrative === "failed" ? "Try again" : it.gist ? "Read it again" : "Read the change"} <Keys k="w" />
              </Button>
            )}
          </div>
        </div>
      )}
    </Row>
  );
}

// KnowLine is one thing worth knowing, under its row: its kind, what it
// is, and the file it rests on, which opens in Files.
function KnowLine({ know: k, repo }: { know: Know; repo: string }) {
  return (
    <span className="team-know-line">
      <KnowChip kind={k.kind} />
      <span>
        <Inline text={k.text} />{" "}
        <Link to={fileHref(k.in || repo, k.file)} className="mono team-know-file" onClick={(e) => e.stopPropagation()} title={`Open ${k.file} in Files`}>
          {k.in ? `${k.in}: ` : ""}
          {k.file.split("/").pop()}
        </Link>
      </span>
    </span>
  );
}

function eventIcon(e: TeamEvent): IconName {
  switch (e.kind) {
    case "opened":
      return "pr";
    case "ready":
      return "eye";
    case "reviewed":
      return e.verdict === "approved" ? "check" : e.verdict === "changes" ? "alert" : "message";
    case "merged":
      return "check-circle";
    case "closed":
      return "x-circle";
    default:
      return "commit";
  }
}

function eventWords(e: TeamEvent): string {
  switch (e.kind) {
    case "opened":
      return "opened it";
    case "ready":
      return "made it ready for review";
    case "reviewed":
      return e.verdict === "approved" ? "approved it" : e.verdict === "changes" ? "asked for changes" : "commented";
    case "merged":
      return "landed it";
    case "closed":
      return "Closed without merging";
    default:
      return `landed, straight on ${e.branch || "the default branch"}: `;
  }
}
