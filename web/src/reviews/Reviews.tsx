import { Keys } from "../ui/Keys";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router";
import { getReviews, type PR, type ReviewRow, useEvents } from "../api";
import { useCommands } from "../keymap/useCommands";
import { reviewsKeys } from "./keys";
import { useReviewsMemory } from "./memory";
import { Icon } from "../ui/Icon";
import { useTitle } from "../ui/useTitle";
import { toast } from "../ui/toast";
import { Loading } from "../ui/Loading";
import { Empty } from "../ui/Empty";
import { Callout } from "../ui/Callout";
import { Time } from "../ui/Time";
import { PullRow } from "./PullRow";
import { ReviewsAsOf } from "./ReviewsAsOf";
import { ReviewsFilter } from "./ReviewsFilter";
import { ReviewsSection } from "./ReviewsSection";
import { useLatest } from "../load";
import { rowTarget } from "./prs";
import { Button } from "../ui/Button";
import { PageHead } from "../ui/PageHead";
import { humanError } from "../ui/strings";
import { Text } from "../ui/Text";

// Reviews: the open pull requests across every repo in the workspace.
// Your teammates' come first, grouped by what each asks of you; yours sit
// below, by where each stands. A stack of pull requests is one row, with
// its layers inside.

// useReviewsEvents calls changed when the list has: it holds the Reviews
// topic on its socket while the page shows, which is also what has the server
// ask GitHub every couple of minutes. Closed, nothing is asked.
export function useReviewsEvents(changed: () => void) {
  useEvents("reviews", { changed });
}

// How the page was left (memory.ts): the list as last read, what was
// selected and folded.
const remember = useReviewsMemory.setState;

export function ReviewsPage() {
  useTitle("Reviews");
  const navigate = useNavigate();
  const list = useReviewsMemory((m) => m.list);
  const foldedRows = useReviewsMemory((m) => m.foldedRows);
  const openSections = useReviewsMemory((m) => m.openSections);
  const [error, setError] = useState<string>();
  const [loading, setLoading] = useState(false);
  const [q, setQ] = useState("");
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
            remember({ list: l });
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
  const sectionOpen = (s: { key: string; folded?: boolean }) => !s.folded || !!openSections[s.key] || !!q;
  const order = sections.filter(sectionOpen).flatMap((s) => s.rows);
  const [selected, setSelected] = useState(() => useReviewsMemory.getState().selected);
  const index = Math.max(0, order.findIndex((r) => r.key === selected));
  const current = order[index];
  const select = (i: number) => {
    const r = order[Math.min(Math.max(i, 0), order.length - 1)];
    if (!r) return;
    setSelected(r.key);
    remember({ selected: r.key });
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
    remember((m) => ({ foldedRows: { ...m.foldedRows, [r.key]: !m.foldedRows[r.key] } }));
  };
  const refresh = () =>
    load(true).then(() => {
      const said = useReviewsMemory.getState().list?.error;
      if (said) toast(humanError(said), { tone: "danger" });
    });
  useCommands(reviewsKeys, {
    next: () => select(index + 1),
    prev: () => select(index - 1),
    first: () => select(0),
    last: () => select(order.length - 1),
    open: () => open(current),
    github: () => github(current),
    fold: () => fold(current),
    refresh,
    filter: () => filter.current?.focus(),
  });

  const head = (
    <PageHead
      title="Reviews"
      actions={
        <>
          {list && <ReviewsAsOf at={list.at} asking={loading} />}
          <ReviewsFilter ref={filter} value={q} onChange={setQ} />
          <Button variant="ghost" size="small" onClick={refresh} disabled={loading} title="Refresh from GitHub" keys={reviewsKeys.refresh}>
            <Icon name="rotate" size={13} /> Refresh <Keys k={reviewsKeys.refresh} />
          </Button>
        </>
      }
    />
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
          <Loading what="Loading pull requests…" />
        )}
      </div>
    );
  }
  const total = list.sections.reduce((n, s) => n + s.rows.length, 0);
  let yoursShown = false;
  return (
    <div className="page">
      {head}
      {(error || list.error) && (
        <Callout tone="attention" title="Could not reach GitHub" actions={<Button onClick={refresh} keys={reviewsKeys.refresh}>Try again</Button>}>
          {humanError(error ?? list.error ?? "")} Last updated <Time iso={list.at} as="relative" />.
        </Callout>
      )}
      {!!list.problems?.length && (
        <Callout tone="quiet" icon="info" title={`${list.problems.length === 1 ? "One thing" : `${list.problems.length} things`} could not be loaded`}>
          {list.problems.map((p) => (
            <div key={p}>{p}</div>
          ))}
        </Callout>
      )}
      {!total && (
        <Empty icon="pr" title="No open pull requests">
          Nothing open in this workspace's repos. Teammates' pull requests show here, then yours.
        </Empty>
      )}
      {total > 0 && !sections.length && <Text as="p" dim>Nothing matches “{q}”.</Text>}
      {sections.map((s) => {
        const isOpen = sectionOpen(s);
        const firstYours = !!s.yours && !yoursShown;
        if (s.yours) yoursShown = true;
        return (
          <ReviewsSection
            key={s.key}
            title={s.title}
            count={s.rows.length}
            folds={s.folded}
            open={isOpen}
            yoursFirst={firstYours}
            onFold={() => remember((m) => ({ openSections: { ...m.openSections, [s.key]: !m.openSections[s.key] } }))}
          >
            {s.rows.map((r) => (
              <PullRow
                key={r.key}
                row={r}
                selected={current?.key === r.key}
                folded={!!foldedRows[r.key]}
                onHover={() => current?.key !== r.key && setSelected(r.key)}
                onOpen={() => {
                  remember({ selected: r.key });
                  open(r);
                }}
                onFold={() => fold(r)}
                passes={list?.passes}
              />
            ))}
          </ReviewsSection>
        );
      })}
    </div>
  );
}
