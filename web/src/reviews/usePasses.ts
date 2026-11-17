import { useCallback, useEffect, useMemo, useState } from "react";
import { getPRGuide, getPRPasses, type ReviewGuide, type ReviewPass, settlePRFinding, type DismissWhy } from "../api";
import { guideOrder } from "./Guide";

// A review's passes and its guide, as its page holds them: read from the
// review's API (a pull request's, or a mission's capsule's: the routes
// are the same under each), read again while one's running, and shaped
// for the diff.

function loadGuided(): boolean {
  try {
    return localStorage.getItem("ws:guide-order") !== "0";
  } catch {
    return true;
  }
}

function loadHidden(): Record<string, boolean> {
  try {
    return JSON.parse(localStorage.getItem("ws:pass-hide") ?? "{}") ?? {};
  } catch {
    return {};
  }
}

export function useReviewPasses(api: string) {
  const [passes, setPasses] = useState<ReviewPass[]>();
  const [guide, setGuide] = useState<ReviewGuide>();
  // Whether the files follow the guide's order, where there's a guide,
  // and the passes whose findings aren't drawn in the diff: both kept in
  // the browser.
  const [guided, setGuided] = useState(loadGuided);
  const [hidden, setHidden] = useState<Record<string, boolean>>(loadHidden);
  // took counts findings taken from the overview: the draft is read again.
  const [took, setTook] = useState(0);

  const loadPasses = useCallback(() => getPRPasses(api).then(setPasses, () => setPasses((ps) => ps ?? [])), [api]);
  // The written guide read again (each second while another's written) is
  // kept as it was, so the files' order made from it is too, and the diff
  // isn't sorted and laid out again each time.
  const loadGuide = useCallback(
    () =>
      getPRGuide(api).then(
        (next) => setGuide((was) => (was?.guide && next.guide && was.at === next.at && was.id === next.id ? { ...next, guide: was.guide } : next)),
        () => setGuide((g) => g ?? {}),
      ),
    [api],
  );
  const putPass = useCallback((p: ReviewPass) => setPasses((ps) => ps?.map((x) => (x.name === p.name ? p : x))), []);
  useEffect(() => {
    setPasses(undefined);
    setGuide(undefined);
    loadPasses();
    loadGuide();
  }, [loadPasses, loadGuide]);
  // While the guide's being written, or a pass is running, look again:
  // what it's doing, and when it's done. The guide's read each second, as
  // it shows what's written so far.
  useEffect(() => {
    if (!guide?.running) return;
    const t = setInterval(loadGuide, 1000);
    return () => clearInterval(t);
  }, [guide?.running, loadGuide]);
  const passing = !!passes?.some((p) => p.running || p.standing.status === "running");
  useEffect(() => {
    if (!passing) return;
    const t = setInterval(loadPasses, 2500);
    return () => clearInterval(t);
  }, [passing, loadPasses]);

  const order = useMemo(
    () => (guide && guided ? guideOrder(guide) : undefined),
    // eslint-disable-next-line react-hooks/exhaustive-deps -- what guideOrder reads
    [guide?.id, guide?.at, guide?.moved, guide?.guide, guided],
  );
  // The guide's line on each file is said on it in either order.
  const notes = useMemo(
    () => (guide ? guideOrder(guide)?.notes : undefined),
    // eslint-disable-next-line react-hooks/exhaustive-deps -- what its notes are read from
    [guide?.guide],
  );
  const setGuidedKept = useCallback((on: boolean) => {
    setGuided(on);
    try {
      localStorage.setItem("ws:guide-order", on ? "1" : "0");
    } catch {
      // fine
    }
  }, []);
  const hide = useCallback(
    (pass: string, on: boolean) =>
      setHidden((h) => {
        const next = { ...h, [pass]: on };
        try {
          localStorage.setItem("ws:pass-hide", JSON.stringify(next));
        } catch {
          // fine
        }
        return next;
      }),
    [],
  );
  // What passes found that's still open, for the diff: kept the same
  // from one render to the next, so the diff isn't laid out again for it.
  const findings = useMemo(
    () => (passes ?? []).filter((p) => !hidden[p.name]).flatMap((p) => p.findings.filter((f) => f.state === "open" && !f.thread).map((f) => ({ pass: p.name, title: p.title, finding: f }))),
    [passes, hidden],
  );
  // What a follow-up said of your comments, by the comment.
  const followups = useMemo(
    () => Object.fromEntries((passes ?? []).flatMap((p) => p.findings.filter((f) => f.thread && f.state === "open").map((f) => [f.thread!, { kind: f.kind, text: f.text, blocks: f.blocks }]))),
    [passes],
  );
  // onFinding takes a finding into the draft, or dismisses it.
  const onFinding = useCallback((pass: string, key: string, what: "taken" | "dismissed", why?: DismissWhy) => settlePRFinding(api, pass, key, what, why).then(putPass), [api, putPass]);
  const bumpTook = useCallback(() => setTook((n) => n + 1), []);

  return { passes, guide, guided, hidden, took, order, notes, findings, followups, loadPasses, loadGuide, putPass, setGuide, setGuidedKept, hide, onFinding, bumpTook };
}
