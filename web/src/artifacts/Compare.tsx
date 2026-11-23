import { useEffect, useMemo, useState } from "react";
import { Link, useLocation, useNavigate, useParams, useSearchParams } from "react-router";
import { type ArtifactDetail, type DiffFile, frameURL, getArtifactCompare, getArtifactDetail, type Workspace } from "../api";
import { useKeys } from "../keys";
import { useLoaded } from "../load";
import { useHighlighter } from "../review/highlight";
import { type Cell, cellHTML, type FileState, fileRows, type Row } from "../review/model";
import { highlight, RowView } from "../review/Review";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/inline";
import { Empty, Loading, useMedia, useTitle } from "../ui/kit";
import { ErrorCard } from "../ui/ErrorCard";
import { Keys } from "../ui/Keys";
import { Option, Select } from "../ui/Select";
import { artifactHref, VersionLine, VersionStrip } from "./Artifacts";
import { compareHref, type Pair, readPair, settle, type Side, sideRef, sideTweaks, stepSide, swapped, withSide } from "./pair";
import { widths } from "./shown";
import { type FrameTweaks, TweaksButton, useTweaks } from "./Tray";
import { followed, frameSearch, type Params, readTweakParams } from "./tweaks";
import { Chip } from "../ui/Chip";
import { Button, buttonClass } from "../ui/Button";
import { Segment, Segmented } from "../ui/Segmented";
import { useWorkspace } from "../workspace/store";

// Comparing: two versions side by side, of one artifact
// (/artifacts/a3/compare?v=1,3) or of two
// (/artifacts/compare?a=a3.v2&b=a5.v1), each in its own sandboxed frame,
// at the same width, with its own versions; Swap puts each where the
// other was. Or their source diff, each file that differs in the diff
// view reviews use, the left the old side. A narrow window shows one at a time. The frames scroll
// apart (the plan's lean on its open question 8).
//
// Each side has its own tweaks (Tray.tsx), behind a button in its head,
// and its own state in the address (a. for the left, b. for the right):
// so one version can be beside itself, as it opens and as it's set
// (/artifacts/a3/compare?v=2,2&b.density=compact).

export const compareKeys: [string, string][] = [
  ["[ / ]", "The right one's version before, or after (on a narrow window, the one shown)"],
  ["{ / }", "The left one's version before, or after"],
  ["S", "Swap them: each where the other was"],
  ["d", "Their source diff, or the frames again"],
  ["t", "On a narrow window, the other one"],
  ["w", "The next width: fit, 1280, 768, 390"],
  ["u", "The diff unified, or side by side"],
  ["r", "Reload: run them again"],
  ["x", "Stop them, or run them again"],
  ["esc", "Back"],
];

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
    <div className="artifact-view compare-view">
      <header className="artifact-head">
        <div className="artifact-title-line">
          <Link to={back} className="ghost icon-button small" aria-label={backLabel} title={backLabel}>
            <Icon name="arrow-left" size={14} />
          </Link>
          <h1>{same ? right.title : `${left.title} and ${right.title}`}</h1>
          <Chip tone="quiet">
            <Icon name="split" size={11} />
            {same ? `v${pair[0].n} and v${pair[1].n}` : "Two artifacts"}
          </Chip>
        </div>
        <div className="artifact-tools">
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
            <Segmented className="compare-which" value={shown ? "right" : "left"} onChange={(v) => setShown(v === "right" ? 1 : 0)} aria-label="Which one's shown">
              {([0, 1] as const).map((i) => (
                <Segment key={i} value={i ? "right" : "left"} title={`${i ? "The right" : "The left"} one (t)`}>
                  {sideRef(pair[i])}
                </Segment>
              ))}
            </Segmented>
          )}
          <span className="artifact-tools-end">
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
          </span>
        </div>
      </header>
      <div className={`compare-body${sides.length === 1 ? " one" : ""}`}>
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
          <div className="compare-diff">
            <SourceDiff pair={pair} split={split} />
          </div>
        ) : (
          sides.map((i) => (
            <div key={`frame${i}`} className="artifact-stage compare-stage">
              <SideFrame side={pair[i]} d={i ? right : left} px={px} run={run} stopped={stopped} onRun={() => setStopped(false)} tweaks={tweaks[i]} opensAs={frameSearch(states[i])} />
            </div>
          ))
        )}
      </div>
    </div>
  );
}

// SideHead is one side's head: which artifact (any of the list), its
// versions, the version's line and note, and Open alone.
function SideHead({
  i,
  pair,
  d,
  choices,
  onPick,
  go,
  keys,
  tweaks,
  opensAs,
  framed,
}: {
  i: 0 | 1;
  pair: Pair;
  d: ArtifactDetail;
  choices: Workspace["artifacts"];
  onPick: (id: string) => void;
  go: (p: Pair) => void;
  keys: [string, string]; // that step its versions
  tweaks: FrameTweaks;
  opensAs: string; // the state it's in, as a frame's address says it
  framed: boolean; // its frame's on the page: not the diff, not stopped
}) {
  const side = pair[i];
  const v = d.all.find((x) => x.n === side.n);
  const runnable = !d.gone && !!v?.size;
  return (
    <div className={`compare-side-head side-${i ? "right" : "left"}`}>
      <div className="compare-side-tools">
        <Select className="compare-pick" value={side.id} onChange={onPick} aria-label={i ? "The right one's artifact" : "The left one's artifact"}>
          {(choices ?? []).map((a) => (
            <Option key={a.id} value={a.id} title={a.title}>
              {`${a.id} · ${a.title}`}
            </Option>
          ))}
        </Select>
        <VersionStrip d={d} n={side.n} go={(n) => go(withSide(pair, i, { id: side.id, n }))} keys={keys} />
        <span className="compare-side-end">
          {framed && <TweaksButton told={tweaks.told} set={tweaks.set} label={i ? "The right one's tweaks" : "The left one's tweaks"} />}
          {runnable && (
            <a className={buttonClass({ variant: "ghost" }, "small compare-alone")} href={frameURL(d.id, side.n, opensAs)} target="_blank" rel="noopener noreferrer" title="In a tab of its own, still sandboxed, as it's set here">
              <Icon name="external" size={13} /> Open alone
            </a>
          )}
        </span>
      </div>
      {v && <VersionLine d={d} v={v} />}
      {v?.note && (
        <div className="artifact-note compare-note">
          <Inline text={v.note} lines={2} />
        </div>
      )}
    </div>
  );
}

// SideFrame is one side's version, running: in its frame, at the width
// both are drawn at. It opens in the state the address gives its side
// when it's made, and is told of changes after (as an artifact's page
// does).
function SideFrame({
  side,
  d,
  px,
  run,
  stopped,
  onRun,
  tweaks,
  opensAs,
}: {
  side: Side;
  d: ArtifactDetail;
  px: number;
  run: number;
  stopped: boolean;
  onRun: () => void;
  tweaks: FrameTweaks;
  opensAs: string;
}) {
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const src = useMemo(() => frameURL(d.id, side.n, opensAs), [d.id, side.n, run, stopped]);
  const v = d.all.find((x) => x.n === side.n);
  if (!v) {
    return (
      <Empty icon="search" title={`${d.id} has no v${side.n}`}>
        Its versions are {d.all.map((x) => `v${x.n}`).join(", ")}.
      </Empty>
    );
  }
  if (d.gone || !v.size) {
    return (
      <Empty icon="alert" title="There's no file to run">
        {d.gone ? `Its folder has gone from ${d.where.replace(/[^/]+\/$/, "")}.` : `v${v.n}'s files aren't in ${d.where}.`}
      </Empty>
    );
  }
  if (stopped) {
    return (
      <Empty icon="stop" title="Stopped" action={<Button onClick={onRun}>Run them</Button>}>
        The frames are unloaded, and their scripts with them.
      </Empty>
    );
  }
  return (
    <div className={`artifact-frame-box${px ? " sized" : ""}`}>
      <iframe
        key={`${d.id}.v${v.n}#${run}`}
        ref={tweaks.frame}
        className="artifact-frame"
        src={src}
        sandbox="allow-scripts"
        referrerPolicy="no-referrer"
        title={`${d.title}, v${v.n}`}
        style={px ? { width: px } : undefined}
      />
    </div>
  );
}

const noop = () => undefined;

// SourceDiff is the left's files against the right's: each file that
// differs, as a review shows a file's diff, under its path. One that's
// its page alone has the one.
function SourceDiff({ pair, split }: { pair: Pair; split: boolean }) {
  const a = sideRef(pair[0]);
  const b = sideRef(pair[1]);
  const { value, error, reload } = useLoaded(() => getArtifactCompare(a, b), [a, b]);
  if (error) return <ErrorCard error={error} retry={reload} title={`Couldn't diff ${a} and ${b}`} />;
  if (!value) return <Loading what="Diffing their sources…" />;
  if (!value.files.length) {
    return (
      <Empty icon="check" title="The same">
        {value.a} and {value.b} are the same, byte for byte.
      </Empty>
    );
  }
  // Named by its path where there's more than the page to tell apart.
  const named = value.files.length > 1 || value.files[0].file.path !== "index.html";
  return (
    <>
      {named && (
        <div className="artifact-diff-files dim">
          {value.files.length} {value.files.length === 1 ? "file differs" : "files differ"} between {value.a} and {value.b}
        </div>
      )}
      {value.files.map((f) => (
        <FileDiff key={`${a}:${b}:${f.file.path}`} file={f.file} lines={f.lines} a={value.a} b={value.b} split={split} named={named} />
      ))}
    </>
  );
}

const statusWords: Record<string, string> = { added: "added", deleted: "removed", modified: "changed" };

// FileDiff is one file's diff between two versions: unified or side by
// side, coloured, with the unchanged lines between the changes there to
// open. A file that isn't text says only that it differs.
function FileDiff({ file: f, lines, a, b, split, named }: { file: DiffFile; lines: string[]; a: string; b: string; split: boolean; named: boolean }) {
  const [expanded, setExpanded] = useState<FileState["expanded"]>({});
  const ready = useHighlighter();
  const state: FileState = useMemo(() => ({ viewed: false, blob: lines, expanded }), [lines, expanded]);
  const rows: Row[] = useMemo(() => fileRows(0, f, state, split), [f, state, split]);
  // ready is a dependency: once the highlighter loads, colour.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const hl = useMemo(() => highlight(f, lines), [f, lines, ready]);
  // expand opens up to n more of a gap's lines on each side (all of it if
  // n is 0), as a review does.
  const expand = (gap: number, n: number) =>
    setExpanded((x) => {
      const e = x[gap] ?? { top: 0, bottom: 0 };
      const first = gap === 0;
      const last = gap === f.hunks.length;
      return { ...x, [gap]: n === 0 ? { top: 1e9, bottom: 0 } : { top: first ? 0 : e.top + n, bottom: last ? 0 : e.bottom + n } };
    });
  const html = (c: Cell, side: "old" | "new") => cellHTML(hl, c, side);
  return (
    <div className="artifact-diff">
      <div className="artifact-diff-head">
        {named && (
          <span className="artifact-diff-path">
            <Icon name="file" size={12} /> {f.path}
            {f.status !== "modified" && <Chip tone={f.status === "added" ? "done" : "quiet"}>{statusWords[f.status] ?? f.status}</Chip>}
          </span>
        )}
        {f.binary ? (
          <span className="dim">It isn't text: it {f.status === "modified" ? "differs" : `was ${statusWords[f.status] ?? f.status}`}.</span>
        ) : (
          <>
            <span>
              <span className="minus">−{f.deletions}</span> in {a}
            </span>
            <span>
              <span className="plus">+{f.additions}</span> in {b}
            </span>
          </>
        )}
      </div>
      {!f.binary && (
        <div className={`lines ${split ? "split" : "unified"} plain`}>
          {rows.slice(1).map((r) => (
            <RowView
              key={r.key}
              row={r}
              split={split}
              cursor={false}
              selected={false}
              html={html}
              onRowClick={noop}
              onGutterDown={noop}
              onDragOver={noop}
              onActivate={(r) => r.type === "gap" && expand(r.gap, r.count !== undefined && r.count <= 40 ? 0 : 20)}
            />
          ))}
        </div>
      )}
    </div>
  );
}
