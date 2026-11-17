import { useEffect, useRef, useState } from "react";
import { docAddress, docTo, fetchSpace, pullSpace, type SpaceActivity, type SpaceDocChange, type SpaceFresh } from "../api";
import { Icon } from "../ui/Icon";
import { humanError, toast } from "../ui/kit";
import { Time } from "../ui/time";
import { TLink } from "../ui/TLink";

// The top of a space, so it reads as somewhere people are working: how
// fresh it is, what's in flight in capsules, and what changed lately on
// its default branch.

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
          <Icon name="spinner" size={12} className="spin" /> Fetching…
        </>
      ) : now.behind ? (
        <>
          <Icon name="alert" size={12} /> {now.behind} {now.behind === 1 ? "commit" : "commits"} behind {remote}
          {now.dirty ? (
            <span> · its ground has changes of its own, so it isn't moved</span>
          ) : (
            <button className="small" onClick={pull} disabled={pulling}>
              {pulling ? "Pulling…" : "Pull"}
            </button>
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

// InFlight is the capsules with changes to a space's documents, each with
// the pull request open from it, and what it changes, read as it is there.
export function InFlight({ repo, activity }: { repo: string; activity: SpaceActivity }) {
  if (!activity.inFlight.length) return null;
  return (
    <section aria-label="In flight">
      <h2 className="group" title="Capsules with changes to this space's documents: yours and others', and the pull requests open from them">
        In flight <span className="count">{activity.inFlight.length}</span>
      </h2>
      <ul className="rows space-flights">
        {activity.inFlight.map((f) => (
          <li key={f.capsule} className="row static">
            <Icon name="box" size={16} className="row-status dim" />
            <span className="row-main">
              <span className="row-line">
                <span className="title mono">{f.capsule}</span>
                {f.mission && <span className="chip tone-quiet">{f.mission}</span>}
                {f.pr && (
                  <TLink to={`/reviews/${encodeURIComponent(repo)}/${f.pr.number}`} className="chip tone-progress" title={f.pr.title}>
                    #{f.pr.number}
                  </TLink>
                )}
              </span>
              <span className="row-line space-changes">
                {f.docs.map((d) => (
                  <Change key={d.path} d={d} to={d.how === "removed" ? undefined : docAddress(`${repo}/${f.capsule}:${d.path}`)} />
                ))}
              </span>
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}

// Lately is the commits on a space's default branch lately that changed
// its documents: who, what they said, when, and which.
export function Lately({ repo, activity }: { repo: string; activity: SpaceActivity }) {
  if (!activity.lately.length) return null;
  return (
    <section aria-label="Changed lately">
      <h2 className="group">Changed lately</h2>
      <ul className="rows space-lately">
        {activity.lately.slice(0, 5).map((c) => (
          <li key={c.sha} className="row static">
            <Icon name="commit" size={16} className="row-status dim" />
            <span className="row-main">
              <span className="row-line">
                <span className="title">{c.subject}</span>
                <span className="dim lately-by">
                  {c.by.split(" ")[0]} · <Time iso={c.at} as="age" />
                </span>
              </span>
              <span className="row-line space-changes">
                {c.docs.slice(0, 6).map((d) => (
                  <Change key={d.path} d={d} to={d.how === "removed" ? undefined : docTo(`${repo}:${d.path}`)} />
                ))}
                {c.docs.length > 6 && <span className="dim">and {c.docs.length - 6} more</span>}
              </span>
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}

function Change({ d, to }: { d: SpaceDocChange; to?: string }) {
  const name = d.title || d.path.split("/").pop();
  const mark = d.how === "added" ? "+" : d.how === "removed" ? "−" : "~";
  return (
    <span className={`space-change ${d.how}`} title={`${d.path}: ${d.how}`}>
      <span className="mono">{mark}</span> {to ? <TLink to={to}>{name}</TLink> : <span className="dim">{name}</span>}
    </span>
  );
}

// inFlightPaths is the paths in flight, and in which capsules.
export function inFlightPaths(activity?: SpaceActivity): Map<string, string[]> {
  const out = new Map<string, string[]>();
  for (const f of activity?.inFlight ?? []) for (const d of f.docs) out.set(d.path, [...(out.get(d.path) ?? []), f.capsule]);
  return out;
}
