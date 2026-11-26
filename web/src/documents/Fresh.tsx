import { useEffect, useRef, useState } from "react";
import { fetchSpace, pullSpace, type SpaceFresh } from "../api";
import { humanError } from "../ui/strings";
import { toast } from "../ui/toast";
import { DocumentsFresh } from "./DocumentsFresh";

// staleAfter is how long a space goes before it's fetched again, when
// it's looked at.
const staleAfter = 5 * 60_000;

// Fresh is how a space's ground stands with its remote: fetched when it's
// looked at (if it hasn't been for a few minutes), said when it's behind,
// and moved forward when you say (a fast-forward of a ground with nothing
// in it, which can't lose anything). It's drawn as a DocumentsFresh.
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
  return <DocumentsFresh fetching={fetching} behind={now.behind} remote={`origin/${now.branch}`} dirty={now.dirty} pulling={pulling} onPull={pull} fetched={now.fetched} />;
}
