import { useEffect, useMemo, useState } from "react";
import { Link, useLocation, useNavigate, useParams, useSearchParams } from "react-router";
import { getArtifactDetail } from "../api";
import { useKeys } from "../keys";
import { useLoaded } from "../load";
import { Button } from "../ui/Button";
import { Chip } from "../ui/Chip";
import { Empty } from "../ui/Empty";
import { ErrorCard } from "../ui/ErrorCard";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import { Loading } from "../ui/Loading";
import { Segment, Segmented } from "../ui/Segmented";
import { useMedia } from "../ui/useMedia";
import { useTitle } from "../ui/useTitle";
import { useWorkspace } from "../workspace/store";
import { ArtifactStage } from "./ArtifactBody";
import { ArtifactCompareBody } from "./ArtifactCompareBody";
import { ArtifactCompareDiff } from "./ArtifactCompareDiff";
import { ArtifactHead } from "./ArtifactHead";
import { ArtifactTitleLine } from "./ArtifactTitleLine";
import { ArtifactTools, ArtifactToolsEnd } from "./ArtifactTools";
import { ArtifactView } from "./ArtifactView";
import { artifactHref } from "./hrefs";
import { compareHref, type Pair, readPair, settle, sideRef, sideTweaks, stepSide, swapped, withSide } from "./pair";
import { widths } from "./shown";
import { SideFrame } from "./SideFrame";
import { SideHead } from "./SideHead";
import { SourceDiff } from "./SourceDiff";
import { followed, frameSearch, type Params, readTweakParams } from "./tweaks";
import { type FrameTweaks, useTweaks } from "./useTweaks";

// Comparing: two versions side by side, of one artifact
// (/artifacts/a3/compare?v=1,3) or of two
// (/artifacts/compare?a=a3.v2&b=a5.v1), each in its own sandboxed frame,
// at the same width, with its own versions; Swap puts each where the
// other was. Or their source diff, each file that differs in the diff
// view reviews use, the left the old side. A narrow window shows one at a time. The frames scroll
// apart (the plan's lean on its open question 8).
//
// Each side has its own tweaks (TweaksButton.tsx), behind a button in its
// head, and its own state in the address (a. for the left, b. for the
// right): so one version can be beside itself, as it opens and as it's set
// (/artifacts/a3/compare?v=2,2&b.density=compact).

// NARROW is a window with room for one frame, as a review's has room for
// one side of its diff.
const NARROW = "(max-width: 860px)";

export function ComparePage() {
  const artifacts = useWorkspace((w) => w.artifacts);
  const { id } = useParams();
  const [search] = useSearchParams();
  const location = useLocation();
  const navigate = useNavigate();
  const asked = useMemo(() => readPair(id, search), [id, search]);
  const ids = asked ? [...new Set(asked.map((s) => s.id))] : [];
  // Read again when the workspace says either has changed: a new version,
  // or where it stands.
  const key = ids
    .map((i) => {
      const b = artifacts?.find((a) => a.id === i);
      return b ? `${i}:${b.versions}:${b.status}:${b.latest?.at ?? ""}:${b.title}` : i;
    })
    .join("|");
  const { value: details, error, reload } = useLoaded(() => Promise.all(ids.map((i) => getArtifactDetail(i))), [key], { keep: true });
  const detail = (i: string) => details?.find((d) => d.id === i);
  const pair = asked && settle(asked, (i) => detail(i)?.all);
  const same = !!pair && pair[0].id === pair[1].id;

  const narrow = useMedia(NARROW);
  const [width, setWidth] = useState(widths[0].key);
  const [run, setRun] = useState(0); // reload: new frames
  const [stopped, setStopped] = useState(false);
  const [diff, setDiff] = useState(false);
  const [splitChosen, setSplit] = useState(true);
  const split = splitChosen && !narrow;
  const [shown, setShown] = useState<0 | 1>(1); // on a narrow window: the right, at first
  // Versions asked for run: changing either starts them again.
  const at = pair ? `${sideRef(pair[0])}:${sideRef(pair[1])}` : "";
  useEffect(() => setStopped(false), [at]);

  const left = pair && detail(pair[0].id);
  const right = pair && detail(pair[1].id);
  const back = same && pair ? artifactHref(pair[1].id, pair[1].n || undefined) : "/artifacts";
  const backLabel = same && right ? `Back to ${right.title}` : "Back to Artifacts";
  useTitle(!pair ? "Compare" : same ? `${right?.title ?? pair[0].id} · v${pair[0].n} and v${pair[1].n}` : `${sideRef(pair[0])} and ${sideRef(pair[1])}`);

  // Each side's tweaks, and the state the address gives it. A side keeps
  // its state while it's the same version: another version, or another
  // artifact, opens as its page says.
  const leftTweaks = useTweaks(pair ? `${sideRef(pair[0])}#${run}` : "");
  const rightTweaks = useTweaks(pair ? `${sideRef(pair[1])}#${run}` : "");
  const tweaks: [FrameTweaks, FrameTweaks] = [leftTweaks, rightTweaks];
  const states: [Params, Params] = [readTweakParams(search, sideTweaks[0]), readTweakParams(search, sideTweaks[1])];
  useEffect(() => {
    const next = followed(search, [
      [sideTweaks[0], leftTweaks.told],
      [sideTweaks[1], rightTweaks.told],
    ]);
    if (next !== undefined) navigate({ search: next }, { replace: true });
  }, [leftTweaks.told, rightTweaks.told, search, navigate]);

  const go = (p: Pair, kept?: [Params, Params]) => {
    if (!pair) return;
    const same = (i: 0 | 1) => p[i].id === pair[i].id && p[i].n === pair[i].n;
    const to = compareHref(p, kept ?? [same(0) ? states[0] : [], same(1) ? states[1] : []]);
    if (to !== location.pathname + location.search) navigate(to, { replace: true });
  };
  const step = (i: 0 | 1, by: -1 | 1) => {
    const d = pair && detail(pair[i].id);
    if (pair && d) go(stepSide(pair, i, by, d.all));
  };
  const swap = () => {
    if (!pair) return;
    // One version beside itself: the frames stay where they are, and what
    // changes sides is the state each is in.
    const [l, r] = [leftTweaks.told, rightTweaks.told];
    if (sideRef(pair[0]) === sideRef(pair[1])) {
      if (l && r) {
        leftTweaks.set(r.state);
        rightTweaks.set(l.state);
      }
      return;
    }
    go(swapped(pair), [states[1], states[0]]);
  };
  const again = () => {
    setStopped(false);
    setRun((x) => x + 1);
  };
  useKeys({
    "[": () => step(narrow ? shown : 1, -1),
    "]": () => step(narrow ? shown : 1, 1),
    "{": () => step(0, -1),
    "}": () => step(0, 1),
    S: swap,
    d: () => setDiff((x) => !x),
    t: () => narrow && setShown((x) => (x ? 0 : 1)),
    w: () => !diff && setWidth((w) => widths[(widths.findIndex((x) => x.key === w) + 1) % widths.length].key),
    u: () => diff && !narrow && setSplit((x) => !x),
    r: () => !diff && again(),
    x: () => !diff && setStopped((s) => !s),
    Escape: () => navigate(back),
  });

  if (!pair) {
    return (
      <div className="page narrow">
        <Empty icon="search" title="Nothing to compare" action={<Link to="/artifacts">Artifacts</Link>}>
          A comparison is of two versions: <code>/artifacts/a3/compare?v=1,3</code>, or of two artifacts, <code>/artifacts/compare?a=a3.v2&amp;b=a5.v1</code>.
        </Empty>
      </div>
    );
  }
  if (error && (!left || !right)) {
    return (
      <div className="page narrow">
        <ErrorCard error={error} retry={reload} title={`Couldn't compare ${sideRef(pair[0])} and ${sideRef(pair[1])}`} />
        <p>
          <Link to="/artifacts">Back to Artifacts</Link>
        </p>
      </div>
    );
  }
  if (!left || !right) return <Loading what="Opening them…" />;

  const px = widths.find((w) => w.key === width)?.px ?? 0;
  const sides: (0 | 1)[] = narrow ? [shown] : [0, 1];
  // What either side can be: the artifacts on the list, and any that's
  // here already (a discarded one, asked for).
  const choices = (artifacts ?? []).filter((a) => a.status !== "discarded" || pair.some((s) => s.id === a.id));
  const pick = (i: 0 | 1, to: string) => {
    if (to === pair[i].id) return;
    go(withSide(pair, i, { id: to, n: artifacts?.find((a) => a.id === to)?.latest?.n ?? 0 }));
  };

  return (
    <ArtifactView>
      <ArtifactHead>
        <ArtifactTitleLine back={{ to: back, label: backLabel }} title={same ? right.title : `${left.title} and ${right.title}`}>
          <Chip tone="quiet">
            <Icon name="split" size={11} />
            {same ? `v${pair[0].n} and v${pair[1].n}` : "Two artifacts"}
          </Chip>
        </ArtifactTitleLine>
        <ArtifactTools>
          <Segmented value={diff ? "diff" : "frames"} onChange={(v) => setDiff(v === "diff")} aria-label="What's compared">
            <Segment value="frames" title="Each running in its frame (d)">
              <Icon name="monitor" size={13} /> Frames
            </Segment>
            <Segment value="diff" title="Their files, the left's against the right's (d)">
              <Icon name="code" size={13} /> Source diff
            </Segment>
          </Segmented>
          {diff ? (
            !narrow && (
              <Segmented value={split ? "split" : "unified"} onChange={(v) => setSplit(v === "split")} aria-label="The diff">
                <Segment value="unified" title="Unified (u)">
                  <Icon name="unified" size={13} /> Unified
                </Segment>
                <Segment value="split" title="Side by side (u)">
                  <Icon name="split" size={13} /> Split
                </Segment>
              </Segmented>
            )
          ) : (
            <Segmented value={width} onChange={setWidth} aria-label="Width">
              {widths.map((w) => (
                <Segment key={w.key} value={w.key} title={`${w.title}, both`}>
                  {w.label}
                </Segment>
              ))}
            </Segmented>
          )}
          {narrow && (
            <Segmented value={shown ? "right" : "left"} onChange={(v) => setShown(v === "right" ? 1 : 0)} aria-label="Which one's shown">
              {([0, 1] as const).map((i) => (
                <Segment key={i} value={i ? "right" : "left"} title={`${i ? "The right" : "The left"} one (t)`}>
                  {sideRef(pair[i])}
                </Segment>
              ))}
            </Segmented>
          )}
          <ArtifactToolsEnd>
            <Button variant="ghost" size="small" onClick={swap} title="Each where the other was: the diff the other way about">
              <Icon name="swap" size={13} /> Swap <Keys k="S" />
            </Button>
            {!diff && (
              <>
                <Button variant="ghost" size="small" onClick={again} title="Run them again">
                  <Icon name="rotate" size={13} /> Reload <Keys k="r" />
                </Button>
                <Button variant="ghost" size="small" onClick={() => setStopped((s) => !s)} title={stopped ? "Run them again" : "Unload them: a script that loops stops"}>
                  <Icon name={stopped ? "play" : "stop"} size={13} /> {stopped ? "Run" : "Stop"} <Keys k="x" />
                </Button>
              </>
            )}
          </ArtifactToolsEnd>
        </ArtifactTools>
      </ArtifactHead>
      <ArtifactCompareBody one={sides.length === 1}>
        {sides.map((i) => (
          <SideHead
            key={`head${i}`}
            i={i}
            pair={pair}
            d={i ? right : left}
            choices={choices}
            onPick={(to) => pick(i, to)}
            go={go}
            keys={narrow || i ? ["[", "]"] : ["{", "}"]}
            tweaks={tweaks[i]}
            opensAs={frameSearch(states[i])}
            framed={!diff && !stopped}
          />
        ))}
        {diff ? (
          <ArtifactCompareDiff>
            <SourceDiff pair={pair} split={split} />
          </ArtifactCompareDiff>
        ) : (
          sides.map((i) => (
            <ArtifactStage key={`frame${i}`} compared>
              <SideFrame side={pair[i]} d={i ? right : left} px={px} run={run} stopped={stopped} onRun={() => setStopped(false)} tweaks={tweaks[i]} opensAs={frameSearch(states[i])} />
            </ArtifactStage>
          ))
        )}
      </ArtifactCompareBody>
    </ArtifactView>
  );
}
