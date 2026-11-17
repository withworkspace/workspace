import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate } from "react-router";
import { getReviews, type PassMark, type PR, type PRState, type ReviewRow, type ReviewsList, type Stack, useEvents } from "../api";
import { useKeys } from "../keys";
import { Icon, type IconName } from "../ui/Icon";
import { Callout, Empty, humanError, Keys, Loading, toast, useTitle } from "../ui/kit";
import { Status, type StatusInfo, type Tone } from "../ui/status";
import { Time } from "../ui/time";
import { useLatest } from "../load";
import { RowPasses } from "./Passes";

// Reviews: the open pull requests across every repo in the workspace.
// Your teammates' come first, grouped by what each asks of you; yours sit
// below, by where each stands. A stack of pull requests is one row, with
// its layers inside.

export const reviewsKeys: [string, string][] = [
  ["j / k", "Next / previous pull request"],
  ["g / G", "First / last"],
  ["⏎", "Open the review (a stack: at the lowest layer that needs you; yours: its mission, or GitHub)"],
  ["o", "Open it on GitHub"],
  ["l", "Fold or unfold a stack's layers"],
  ["/", "Filter by text, repo or author"],
  ["r", "Ask GitHub again"],
];

const stateInfo: Record<PRState, { icon: IconName; tone: Tone }> = {
  "needs-review": { icon: "circle-dot", tone: "attention" },
  "back-with-you": { icon: "reply", tone: "attention" },
  "waiting-on-them": { icon: "clock", tone: "waiting" },
  approved: { icon: "check-circle", tone: "done" },
  others: { icon: "circle", tone: "quiet" },
  drafts: { icon: "circle-dashed", tone: "quiet" },
  automated: { icon: "sparkle", tone: "quiet" },
  returned: { icon: "alert", tone: "attention" },
  ready: { icon: "check-circle", tone: "attention" },
  "waiting-review": { icon: "clock", tone: "waiting" },
};

const capital = (s: string) => (s ? s[0].toUpperCase() + s.slice(1) : s);

// prStatus is where a pull request stands with you.
export function prStatus(p: { state: PRState; words: string }): StatusInfo {
  return { ...(stateInfo[p.state] ?? { icon: "circle", tone: "quiet" }), words: capital(p.words) };
}

// prHref is a pull request's review here.
export const prHref = (repo: string, number: number) => `/reviews/${encodeURIComponent(repo)}/${number}`;

// useReviewsEvents calls changed when the list has: it holds the Reviews
// event stream while the page shows, which is also what has the server
// ask GitHub every couple of minutes. Closed, nothing is asked.
export function useReviewsEvents(changed: () => void) {
  useEvents("reviews", { changed });
}

// The list as last fetched, kept across visits so coming back shows it at
// once; and what was selected and folded.
let kept: ReviewsList | undefined;
let lastSelected = "";
const foldedRows = new Set<string>();
const openSections = new Set<string>();

export function ReviewsPage() {
  useTitle("Reviews");
  const navigate = useNavigate();
  const [list, setList] = useState(kept);
  const [error, setError] = useState<string>();
  const [loading, setLoading] = useState(false);
  const [q, setQ] = useState("");
  const [, redraw] = useState(0);
  const filter = useRef<HTMLInputElement>(null);

  const begin = useLatest();
  const load = useCallback(
    (refresh = false) => {
      const fresh = begin();
      setLoading(true);
      return getReviews(refresh)
        .then(
          (l) => {
            if (!fresh()) return;
            kept = l;
            setList(l);
            setError(undefined);
          },
          (e: Error) => fresh() && setError(e.message),
        )
        .finally(() => fresh() && setLoading(false));
    },
    [begin],
  );
  useEffect(() => {
    load();
  }, [load]);
  useReviewsEvents(load);

  // What the filter leaves: rows whose title, repo, author, branch or
  // number has every word.
  const sections = useMemo(() => {
    const words = q.toLowerCase().split(/\s+/).filter(Boolean);
    const has = (p: PR) => {
      const hay = `${p.title} ${p.repo} ${p.author} ${p.branch} #${p.number} ${p.repo}#${p.number}`.toLowerCase();
      return words.every((w) => hay.includes(w));
    };
    const keep = (r: ReviewRow) => !words.length || (r.pr ? has(r.pr) : r.stack!.layers.some(has));
    return (list?.sections ?? []).map((s) => ({ ...s, rows: s.rows.filter(keep) })).filter((s) => s.rows.length);
  }, [list, q]);
  // Folded sections (drafts, bots) open on a click, or when filtering.
  const sectionOpen = (s: { key: string; folded?: boolean }) => !s.folded || openSections.has(s.key) || !!q;
  const order = sections.filter(sectionOpen).flatMap((s) => s.rows);
  const [selected, setSelected] = useState(lastSelected);
  const index = Math.max(0, order.findIndex((r) => r.key === selected));
  const current = order[index];
  const select = (i: number) => {
    const r = order[Math.min(Math.max(i, 0), order.length - 1)];
    if (!r) return;
    setSelected(r.key);
    lastSelected = r.key;
    document.getElementById(`pr-row-${r.key}`)?.scrollIntoView({ block: "nearest" });
  };
  const open = (r?: ReviewRow) => {
    if (!r) return;
    const to = rowTarget(r);
    if (to.href) window.open(to.href, "_blank", "noopener");
    else navigate(to.to!);
  };
  const github = (r?: ReviewRow) => {
    const pr = r?.pr ?? r?.stack?.layers.find((l) => l.number === r.stack!.open) ?? r?.stack?.layers[0];
    if (pr) window.open(pr.url, "_blank", "noopener");
  };
  const fold = (r?: ReviewRow) => {
    if (!r?.stack) return;
    if (!foldedRows.delete(r.key)) foldedRows.add(r.key);
    redraw((n) => n + 1);
  };
  const refresh = () =>
    load(true).then(() => {
      if (kept?.error) toast(humanError(kept.error), { tone: "danger" });
    });
  useKeys({
    j: () => select(index + 1),
    ArrowDown: () => select(index + 1),
    k: () => select(index - 1),
    ArrowUp: () => select(index - 1),
    g: () => select(0),
    G: () => select(order.length - 1),
    Enter: () => open(current),
    o: () => github(current),
    l: () => fold(current),
    r: refresh,
    "/": () => filter.current?.focus(),
  });

  const head = (
    <header className="page-head">
      <h1>Reviews</h1>
      <div className="page-actions">
        {list && (
          <span className="dim reviews-asked" title="When GitHub was last asked. While this page is open it's asked every couple of minutes.">
            {loading ? (
              <>
                <Icon name="spinner" className="spin" size={12} /> Asking GitHub…
              </>
            ) : (
              <>
                As of <Time iso={list.at} as="relative" />
              </>
            )}
          </span>
        )}
        <input
          ref={filter}
          className="reviews-filter"
          value={q}
          placeholder="Filter: text, repo, author"
          aria-label="Filter pull requests"
          onChange={(e) => setQ(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Escape" || e.key === "Enter") {
              e.preventDefault();
              if (e.key === "Escape") setQ("");
              e.currentTarget.blur();
            }
          }}
        />
        <button className="ghost small" onClick={refresh} disabled={loading} title="Ask GitHub again (r)">
          <Icon name="rotate" size={13} /> Refresh <Keys k="r" />
        </button>
      </div>
    </header>
  );

  if (!list) {
    return (
      <div className="page">
        {head}
        {error ? (
          <Empty icon="pr" title="No pull requests to show">
            {humanError(error)}
          </Empty>
        ) : (
          <Loading what="Asking GitHub for the open pull requests…" />
        )}
      </div>
    );
  }
  const total = list.sections.reduce((n, s) => n + s.rows.length, 0);
  let yoursShown = false;
  return (
    <div className="page reviews-page">
      {head}
      {(error || list.error) && (
        <Callout tone="attention" title="GitHub couldn't be asked just now" actions={<button onClick={refresh}>Try again</button>}>
          {humanError(error ?? list.error ?? "")} What's here is as of <Time iso={list.at} as="relative" />.
        </Callout>
      )}
      {!!list.problems?.length && (
        <Callout tone="quiet" icon="info" title={`${list.problems.length === 1 ? "One thing" : `${list.problems.length} things`} couldn't be read`}>
          {list.problems.map((p) => (
            <div key={p}>{p}</div>
          ))}
        </Callout>
      )}
      {!total && (
        <Empty icon="pr" title="No open pull requests">
          Nothing is open in this workspace's repos. Your teammates' pull requests show here when there are some, by what each asks of you, and yours below them.
        </Empty>
      )}
      {total > 0 && !sections.length && <p className="dim">Nothing matches “{q}”.</p>}
      {sections.map((s) => {
        const isOpen = sectionOpen(s);
        const firstYours = !!s.yours && !yoursShown;
        if (s.yours) yoursShown = true;
        return (
          <section key={s.key} aria-label={s.title} className={s.yours ? "yours" : undefined}>
            {firstYours && <div className="reviews-divide">Yours</div>}
            <h2 className="group">
              {s.folded ? (
                <button
                  className="link group-fold"
                  aria-expanded={isOpen}
                  onClick={() => {
                    if (!openSections.delete(s.key)) openSections.add(s.key);
                    redraw((n) => n + 1);
                  }}
                >
                  <Icon name={isOpen ? "chevron-down" : "chevron-right"} size={12} /> {s.title}
                </button>
              ) : (
                s.title
              )}{" "}
              <span className="count">{s.rows.length}</span>
            </h2>
            {isOpen && (
              <ul className="rows prs" role="listbox" aria-label={s.title}>
                {s.rows.map((r) => (
                  <RowView
                    key={r.key}
                    row={r}
                    selected={current?.key === r.key}
                    folded={foldedRows.has(r.key)}
                    onHover={() => current?.key !== r.key && setSelected(r.key)}
                    onOpen={() => {
                      lastSelected = r.key;
                      open(r);
                    }}
                    onFold={() => fold(r)}
                    passes={list?.passes}
                  />
                ))}
              </ul>
            )}
          </section>
        );
      })}
    </div>
  );
}

// rowTarget is where a row opens: a teammate's, its review here (a stack
// at the lowest layer that needs you); yours, its mission, which already
// says what moves it on, or with none, GitHub, where it's merged.
function rowTarget(r: ReviewRow): { to?: string; href?: string } {
  const pr = r.pr ?? r.stack!.layers.find((l) => l.number === r.stack!.open) ?? r.stack!.layers[0];
  if (pr.mine) {
    const mission = r.pr ? r.pr.mission : r.stack!.layers.find((l) => l.mission)?.mission;
    return mission ? { to: `/missions/${mission.slug}` } : { href: pr.url };
  }
  return { to: prHref(pr.repo, pr.number) };
}

function Size({ additions, deletions, files }: { additions: number; deletions: number; files?: number }) {
  return (
    <span className="row-meta pr-size" title={files !== undefined ? `${files} ${files === 1 ? "file" : "files"} changed` : undefined}>
      <span className="plus">+{additions}</span> <span className="minus">−{deletions}</span>
      {files !== undefined && (
        <span className="dim">
          {files} {files === 1 ? "file" : "files"}
        </span>
      )}
    </span>
  );
}

// Checks is a pull request's checks, as a mark: passing, failing, running.
export function Checks({ checks }: { checks?: PR["checks"] }) {
  if (!checks) return <span className="pr-checks none" />;
  const [icon, words]: [IconName, string] = checks === "success" ? ["check", "Checks pass"] : checks === "failure" ? ["x", "Checks are failing"] : ["clock", "Checks are running"];
  return (
    <span className={`pr-checks c-${checks}`} title={words}>
      <Icon name={icon} size={12} />
      <span className="sr-only">{words}</span>
    </span>
  );
}

interface RowProps {
  row: ReviewRow;
  selected: boolean;
  folded: boolean;
  onHover: () => void;
  onOpen: () => void;
  onFold: () => void;
  passes?: Record<string, PassMark[]>; // how passes stand, by pull request
}

function RowView({ row, selected, folded, onHover, onOpen, onFold, passes }: RowProps) {
  const target = rowTarget(row);
  const pr = row.pr;
  const st = row.stack;
  const status = prStatus(pr ?? st!);
  // (A field GitHub left out is absent, so each is read from whichever
  // the row is, not from the first that has it.)
  const of = <T,>(a: (p: PR) => T, b: (s: Stack) => T): T => (pr ? a(pr) : b(st!));
  const repo = of((p) => p.repo, (s) => s.repo);
  const author = of((p) => p.author, (s) => s.author);
  const mine = of((p) => p.mine, (s) => s.mine);
  const mission = of((p) => p.mission, (s) => s.layers.find((l) => l.mission)?.mission);
  const updated = of((p) => p.updated, (s) => s.layers.reduce((a, l) => (l.updated && l.updated > a ? l.updated : a), ""));
  const onTopOf = of((p) => p.onTopOf, (s) => s.onTopOf);
  const body = (
    <>
      <Status s={status} iconOnly size={16} className="row-status" />
      <span className="row-main">
        <span className="row-line">
          <span className="title">{of((p) => p.title, (s) => s.title)}</span>
          {st && (
            <span className="chip tone-quiet" title={st.workedOut ? "A stack ws worked out: each of these pull requests is based on the last one's branch" : `Stack ${st.number} on GitHub`}>
              <Icon name="layers" size={11} /> stack of {st.layers.length}
            </span>
          )}
          {pr?.draft && <span className="chip tone-quiet">Draft</span>}
          {pr?.asked === "team" && (
            <span className="chip tone-quiet" title="Asked of a team you're in">
              Your team
            </span>
          )}
          {!!onTopOf && (
            <span className="chip tone-quiet" title="Its base is that pull request's branch">
              on top of #{onTopOf}
            </span>
          )}
          {mission && (
            <span className="chip tone-quiet" title={`From the mission ${mission.title}`}>
              <Icon name="flag" size={11} /> {mission.title}
            </span>
          )}
          {pr?.capsule && (
            <span className="chip tone-quiet" title={`Checked out here in ${pr.repo}/${pr.capsule}`}>
              <Icon name="box" size={11} /> {pr.capsule}
            </span>
          )}
          {pr && <RowPasses marks={passes?.[`${pr.repo}#${pr.number}`]} />}
        </span>
        <span className="row-sub">
          {mine ? "You" : author} · {repo}
          {pr && <> · #{pr.number}</>} · {of((p) => p.words, (s) => s.words)}
          {st?.merged ? ` · ${st.merged} merged below` : ""}
        </span>
      </span>
      <Size additions={of((p) => p.additions, (s) => s.additions)} deletions={of((p) => p.deletions, (s) => s.deletions)} files={pr?.files} />
      <Checks checks={pr?.checks} />
      {updated ? <Time iso={updated} as="age" className="age" /> : <span className="age" />}
    </>
  );
  return (
    <li id={`pr-row-${row.key}`} role="option" aria-selected={selected} className={`row pr-row${selected ? " selected" : ""}${st ? " stack-row" : ""}`} onMouseMove={onHover}>
      {target.to ? (
        <Link to={target.to} className="row-link" onClick={() => (lastSelected = row.key)}>
          {body}
        </Link>
      ) : (
        <a
          href={target.href}
          target="_blank"
          rel="noreferrer"
          className="row-link"
          title="Opens on GitHub: it's merged there"
          onClick={(e) => {
            e.preventDefault();
            onOpen();
          }}
        >
          {body}
        </a>
      )}
      {st && (
        <button className="ghost icon-button small stack-fold" onClick={onFold} title={`${folded ? "Show" : "Hide"} its layers (l)`} aria-expanded={!folded} aria-label="Its layers">
          <Icon name={folded ? "chevron-right" : "chevron-down"} size={13} />
        </button>
      )}
      {st && !folded && <Layers stack={st} passes={passes} />}
    </li>
  );
}

// Layers are a stack's pull requests, bottom first: the order to read
// them in. Each has its own state, and opens its own review.
function Layers({ stack, passes }: { stack: Stack; passes?: Record<string, PassMark[]> }) {
  return (
    <ol className="stack-layers">
      {stack.layers.map((l) => {
        const s = prStatus(l);
        const inner = (
          <>
            <Status s={s} iconOnly size={13} />
            <span className="layer-title">{l.title}</span>
            <span className="mono dim">#{l.number}</span>
            <span className={`layer-words tone-${s.tone}`}>{l.words}</span>
            <RowPasses marks={passes?.[`${l.repo}#${l.number}`]} />
            <span className="spacer" />
            <span className="pr-size">
              <span className="plus">+{l.additions}</span> <span className="minus">−{l.deletions}</span>
            </span>
            <Checks checks={l.checks} />
          </>
        );
        return (
          <li key={l.number}>
            {l.mine && !l.mission ? (
              <a href={l.url} target="_blank" rel="noreferrer" className="layer-link">
                {inner}
              </a>
            ) : (
              <Link to={l.mine && l.mission ? `/missions/${l.mission.slug}` : prHref(l.repo, l.number)} className="layer-link">
                {inner}
              </Link>
            )}
          </li>
        );
      })}
    </ol>
  );
}
