import { type ReactNode, useEffect, useRef, useState } from "react";
import { docAddress, docTo, fetchSpace, pullSpace, type Space, type SpaceActivity, type SpaceCommit, type SpaceDocChange, type SpaceFlight, type SpaceFresh } from "../api";
import { Icon } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import { toast } from "../ui/toast";
import { humanError } from "../ui/text";
import { Time } from "../ui/time";
import { TLink } from "../ui/TLink";
import { changedSince, type InSpace, saysNoMore } from "./happening";
import { Chip, chipClass } from "../ui/Chip";
import { Button } from "../ui/Button";
import { GroupHead } from "../ui/GroupHead";
import { Row, RowLine, RowMain, RowTitle, Rows, rowStatusClass } from "../ui/Rows";

// What's happening in a space, so it reads as somewhere people are
// working: how fresh it is, what's in flight in capsules, and what changed
// lately on its default branch. A space's page has it folded to a line
// over its list; Documents has every space's above the spaces.

// staleAfter is how long a space goes before it's fetched again, when
// it's looked at.
const staleAfter = 5 * 60_000;

// Fresh is how a space's ground stands with its remote: fetched when it's
// looked at (if it hasn't been for a few minutes), said when it's behind,
// and moved forward when you say (a fast-forward of a ground with nothing
// in it, which can't lose anything).
export function Fresh({ repo, fresh, onMoved }: { repo: string; fresh: SpaceFresh; onMoved: () => void }) {
  const [now, setNow] = useState<SpaceFresh>(fresh);
  const [fetching, setFetching] = useState(false);
  const [pulling, setPulling] = useState(false);
  const asked = useRef("");
  useEffect(() => setNow(fresh), [fresh]);
  useEffect(() => {
    const stale = !fresh.fetched || Date.now() - new Date(fresh.fetched).getTime() > staleAfter;
    if (!stale || asked.current === repo) return;
    asked.current = repo;
    setFetching(true);
    fetchSpace(repo)
      .then((r) => setNow(r.fresh), () => {}) // offline: it's as last fetched
      .finally(() => setFetching(false));
  }, [repo, fresh.fetched]);
  const pull = () => {
    setPulling(true);
    pullSpace(repo)
      .then(
        (r) => {
          setNow(r.fresh);
          toast("Up to date", { tone: "done" });
          onMoved();
        },
        (e: Error) => toast(humanError(e.message), { tone: "danger" }),
      )
      .finally(() => setPulling(false));
  };
  const remote = `origin/${now.branch}`;
  return (
    <p className={`space-fresh dim${now.behind ? " behind" : ""}`}>
      {fetching ? (
        <>
          <Spinner size={12} /> Fetching…
        </>
      ) : now.behind ? (
        <>
          <Icon name="alert" size={12} /> {now.behind} {now.behind === 1 ? "commit" : "commits"} behind {remote}
          {now.dirty ? (
            <span> · its ground has changes of its own, so it isn't moved</span>
          ) : (
            <Button size="small" onClick={pull} disabled={pulling}>
              {pulling ? "Pulling…" : "Pull"}
            </Button>
          )}
        </>
      ) : (
        <>
          <Icon name="check" size={12} /> Up to date with {remote}
        </>
      )}
      {now.fetched && !fetching && (
        <span>
          {" "}
          · fetched <Time iso={now.fetched} as="age" /> ago
        </span>
      )}
    </p>
  );
}

// InFlight is the capsules with changes to a space's documents (or to
// any space's, each with its space), each with the pull request open from
// it, and what it changes, read as it is there.
export function InFlight({ flights, spaceOf }: { flights: InSpace<SpaceFlight>[]; spaceOf?: (repo: string) => Space | undefined }) {
  if (!flights.length) return null;
  return (
    <section aria-label="In flight">
      <GroupHead className="activity-head" title="Capsules with changes to the documents: yours and others', and the pull requests open from them" count={flights.length}>
        In flight
      </GroupHead>
      <Rows className="space-flights">
        {flights.map((f) => {
          const space = spaceOf?.(f.repo);
          return (
            <Row still className="flight-row" key={`${f.repo}:${f.capsule}`}>
              <Icon name="box" size={16} className={rowStatusClass("dim")} />
              <RowMain className="flight-main">
                <RowLine>
                  <RowTitle className="mono">{f.capsule}</RowTitle>
                  {f.mission && <Chip tone="quiet">{f.mission}</Chip>}
                  {f.pr && (
                    <TLink to={`/reviews/${encodeURIComponent(f.repo)}/${f.pr.number}`} className={chipClass("progress")} title={f.pr.title}>
                      #{f.pr.number}
                    </TLink>
                  )}
                  {space && <SpaceTag space={space} />}
                </RowLine>
                <RowLine className="space-changes">
                  {f.docs.map((d) => (
                    <Change key={d.path} d={d} to={d.how === "removed" ? undefined : docAddress(`${f.repo}/${f.capsule}:${d.path}`)} />
                  ))}
                </RowLine>
              </RowMain>
            </Row>
          );
        })}
      </Rows>
    </section>
  );
}

// Lately is the commits on the default branch lately that changed
// documents (a space's, or every space's, each with its space), a line
// each: which document, what was done to it, who and when; what the
// commit said only when it says more than that.
export function Lately({ lately, limit = 5, spaceOf, label = "Changed lately" }: { lately: InSpace<SpaceCommit>[]; limit?: number; spaceOf?: (repo: string) => Space | undefined; label?: ReactNode }) {
  if (!lately.length) return null;
  const shown = lately.slice(0, limit);
  return (
    <section aria-label="Changed lately">
      <GroupHead className="activity-head" count={lately.length}>
        {label}
      </GroupHead>
      <ul className="doc-changes">
        {shown.map((c) => (
          <ChangeLine key={`${c.repo}:${c.sha}`} c={c} space={spaceOf?.(c.repo)} />
        ))}
        {lately.length > shown.length && <li className="doc-changes-more dim">and {lately.length - shown.length === 1 ? "1 earlier change" : `${lately.length - shown.length} earlier changes`}</li>}
      </ul>
    </section>
  );
}

// ChangeLine is a commit on one line, by the first document it changed.
export function ChangeLine({ c, space }: { c: InSpace<SpaceCommit>; space?: Space }) {
  const d = c.docs[0];
  if (!d) return null;
  const name = d.title || d.path.split("/").pop();
  const rest = c.docs.slice(1);
  return (
    <li className="doc-change">
      <span className={`doc-change-how ${d.how}`} title={d.how}>
        {mark(d.how)}
      </span>
      {d.how === "removed" ? (
        <span className="doc-change-title dim">{name}</span>
      ) : (
        <TLink to={docTo(`${c.repo}:${d.path}`)} className="doc-change-title" title={d.path}>
          {name}
        </TLink>
      )}
      {rest.length > 0 && (
        <span className="doc-change-more" title={rest.map((r) => `${mark(r.how)} ${r.title || r.path}`).join("\n")}>
          +{rest.length} more
        </span>
      )}
      <span className="doc-change-what">{d.how}</span>
      {!saysNoMore(c.subject, d) && (
        <span className="doc-change-subject" title={c.subject}>
          {c.subject}
        </span>
      )}
      {space && <SpaceTag space={space} />}
      <span className="doc-change-by" title={c.by}>
        {c.by.split(" ")[0]} · <Time iso={c.at} as="age" />
      </span>
    </li>
  );
}

// ActivityFold is a space's activity folded to one line (how many in
// flight, how many changes this week, the latest), open for In flight and
// Changed lately side by side.
export function ActivityFold({ repo, activity, open, setOpen }: { repo: string; activity: SpaceActivity; open: boolean; setOpen: (open: boolean) => void }) {
  const lately = activity.lately.map((c) => ({ ...c, repo }));
  const flights = activity.inFlight.map((f) => ({ ...f, repo }));
  if (!lately.length && !flights.length) return null;
  const week = changedSince(activity.lately, Date.now() - 7 * 24 * 3600_000);
  const latest = activity.lately[0];
  return (
    <details className="activity-fold" open={open} onToggle={(e) => setOpen(e.currentTarget.open)}>
      <summary>
        <Icon name="clock" size={14} className="dim" />
        <span>
          {flights.length > 0 && (
            <>
              <strong>{flights.length}</strong> in flight
            </>
          )}
          {flights.length > 0 && week > 0 && " · "}
          {week > 0 && (
            <>
              <strong>{week}</strong> {week === 1 ? "change" : "changes"} this week
            </>
          )}
          {!flights.length && !week && "Nothing this week"}
        </span>
        {latest && (
          <span className="activity-fold-latest">
            · latest: {latest.docs[0]?.title || latest.subject}, {latest.by.split(" ")[0]} <Time iso={latest.at} as="age" /> ago
          </span>
        )}
        <Icon name="chevron-down" size={14} className="dim activity-fold-caret" />
      </summary>
      <div className="activity-fold-body two-up">
        <InFlight flights={flights} />
        <Lately lately={lately} />
      </div>
    </details>
  );
}

// SpaceTag is which space something's in, where there's more than one.
function SpaceTag({ space }: { space: Space }) {
  return (
    <span className="space-tag">
      <span className="repo-dot" style={{ background: space.color }} />
      {space.label}
    </span>
  );
}

const mark = (how: SpaceDocChange["how"]) => (how === "added" ? "+" : how === "removed" ? "−" : "~");

function Change({ d, to }: { d: SpaceDocChange; to?: string }) {
  const name = d.title || d.path.split("/").pop();
  return (
    <span className={`space-change ${d.how}`} title={`${d.path}: ${d.how}`}>
      <span className="mono">{mark(d.how)}</span> {to ? <TLink to={to}>{name}</TLink> : <span className="dim">{name}</span>}
    </span>
  );
}

// inFlightPaths is the paths in flight, and in which capsules.
export function inFlightPaths(activity?: SpaceActivity): Map<string, string[]> {
  const out = new Map<string, string[]>();
  for (const f of activity?.inFlight ?? []) for (const d of f.docs) out.set(d.path, [...(out.get(d.path) ?? []), f.capsule]);
  return out;
}
