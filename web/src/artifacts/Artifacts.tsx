import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router";
import { type ArtifactDetail, type ArtifactInfo, type ArtifactVersion, frameURL, getArtifactDetail, getArtifactSource, type Workspace } from "../api";
import { useKeys } from "../keys";
import { useLoaded } from "../load";
import { RefTag } from "../refs/RefTag";
import { refKey, refName } from "../refs/refs";
import { escapeHTML, highlightLines, languageOf, useHighlighter } from "../review/highlight";
import { HoverCard } from "../ui/hover";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/inline";
import { Copyable, Empty, ErrorCard, Keys, Loading, useTitle } from "../ui/kit";
import { Option, Select } from "../ui/Select";
import { Status } from "../ui/status";
import { Time } from "../ui/time";
import { TLink } from "../ui/TLink";
import {
  ANYWHERE,
  artifactStatus,
  BY_HAND,
  byWords,
  expiryOf,
  KEEP_DISCARDED,
  fromRef,
  isConversation,
  keptWords,
  keptIcon,
  keptTitle,
  inFlight,
  missionTalk,
  listed,
  type Shown,
  shownLabels,
  sizeWords,
  sources,
  stepVersion,
  versionOf,
  widths,
} from "./shown";
import { compareFrom, compareHref } from "./pair";
import { ClearExpired, DiscardDialog, PromoteDialog, restore } from "./Lifetime";

// Artifacts: versioned HTML prototypes, listed, and each run in a
// sandboxed frame. A version runs from a route of its own (/frame/a3/v2),
// in <iframe sandbox="allow-scripts">: its scripts run, in an origin of
// their own, so they can't read ws's cookie, its storage or this page,
// and its policy lets nothing leave.

export const artifactsKeys: [string, string][] = [
  ["j / k", "Move down and up the list"],
  ["⏎ or o", "Open the artifact"],
];

export const artifactKeys: [string, string][] = [
  ["[ / ]", "The version before, or after"],
  ["w", "The next width: fit, 1280, 768, 390"],
  ["r", "Reload: run it again"],
  ["x", "Stop it, or run it again"],
  ["s", "Its source, or the frame again"],
  ["c", "Compare it, side by side: beside the version it's based on"],
  ["p", "Promote it: keep it for good, choosing this version"],
  ["O", "Open it alone, in a tab of its own"],
  ["esc", "Back to Artifacts"],
];

export const artifactHref = (id: string, n?: number) => (n ? `/artifacts/${id}/v${n}` : `/artifacts/${id}`);

// From is where a version came from: a conversation or a mission, drawn
// as its ref where this machine has it, its title where it hasn't (one
// made on another machine).
function From({ from, title, workspace, inline }: { from: string; title?: string; workspace: Workspace; inline?: boolean }) {
  const talk = missionTalk(from);
  const here = isConversation(from) ? workspace.conversations.some((t) => t.id === from) : workspace.missions.some((m) => m.slug === (talk?.slug ?? from));
  if (!here) {
    return (
      <span className="artifact-from" title={title ? `${from}: ${title}` : from}>
        {title ? `“${title}”` : from}
      </span>
    );
  }
  const r = fromRef(from);
  if (talk)
    return (
      <span title={title ? `${talk.id} on ${talk.slug}: ${title}` : undefined}>
        <TLink to={`/missions/${talk.slug}/conversations/${talk.id}`} className="link">
          {talk.id}
        </TLink>{" "}
        on{" "}
        <RefTag refKey={refKey(r)} inline={inline}>
          {refName(r)}
        </RefTag>
      </span>
    );
  return (
    <RefTag refKey={refKey(r)} inline={inline}>
      {refName(r)}
    </RefTag>
  );
}

// --- The list

export function ArtifactsPage({ workspace }: { workspace: Workspace }) {
  useTitle("Artifacts");
  const navigate = useNavigate();
  const all = useMemo(() => workspace.artifacts ?? [], [workspace.artifacts]);
  const [shown, setShown] = useState<Shown>("all");
  const [from, setFrom] = useState(ANYWHERE);
  const list = useMemo(() => listed(all, shown, from), [all, shown, from]);
  const froms = useMemo(() => sources(all), [all]);
  const discarded = all.filter((a) => a.status === "discarded").length;
  const [index, setIndex] = useState(0);
  const [promoting, setPromoting] = useState<ArtifactInfo>();
  const move = (i: number) => {
    const n = Math.min(Math.max(i, 0), list.length - 1);
    setIndex(n);
    document.getElementById(`artifact-${list[n]?.id}`)?.scrollIntoView({ block: "nearest" });
  };
  const open = () => list[index] && navigate(artifactHref(list[index].id));
  useKeys({ j: () => move(index + 1), ArrowDown: () => move(index + 1), k: () => move(index - 1), ArrowUp: () => move(index - 1), Enter: open, o: open });

  return (
    <div className="page narrow artifacts-page">
      <header className="page-head">
        <h1>Artifacts</h1>
        {all.length > 0 && (
          <div className="page-actions">
            <div className="segmented" role="group" aria-label="Where they stand">
              {(Object.keys(shownLabels) as Shown[]).map((s) => (
                <button key={s} aria-pressed={shown === s} onClick={() => setShown(s)}>
                  {shownLabels[s]}
                  {s === "discarded" && discarded > 0 ? ` ${discarded}` : ""}
                </button>
              ))}
            </div>
            {froms.length > 0 && (
              <Select value={from} onChange={setFrom} aria-label="Where they came from">
                <Option value={ANYWHERE}>From anywhere</Option>
                <Option value={BY_HAND}>Added by hand</Option>
                {froms.map((f) => (
                  <Option key={f.from} value={f.from} title={f.title}>
                    {f.title ? `${f.from}: ${f.title}` : f.from}
                  </Option>
                ))}
              </Select>
            )}
            <ClearExpired all={all} workspace={workspace} />
          </div>
        )}
      </header>
      {all.length === 0 ? (
        <Empty icon="artifact" title="No artifacts yet">
          An artifact is a sketch to look at and use: one HTML file, with its CSS and JS inline, in versions you can step through. Crew add them with{" "}
          <code>ws artifact add</code>, and so can you.
        </Empty>
      ) : list.length === 0 ? (
        <Empty icon="search" title="None here">
          No artifact {shown === "all" ? "" : `that's ${shownLabels[shown].toLowerCase()} `}came from there.
        </Empty>
      ) : (
        <ul className="rows artifacts" role="listbox" aria-label="Artifacts">
          {list.map((a, i) => (
            <ArtifactListRow key={a.id} a={a} selected={i === index} onHover={() => i !== index && setIndex(i)} workspace={workspace} onPromote={() => setPromoting(a)} />
          ))}
        </ul>
      )}
      {promoting && <PromoteDialog a={promoting} workspace={workspace} onClose={() => setPromoting(undefined)} />}
    </div>
  );
}

function ArtifactListRow({ a, selected, onHover, workspace, onPromote }: { a: ArtifactInfo; selected: boolean; onHover: () => void; workspace: Workspace; onPromote: () => void }) {
  const v = a.latest;
  // A live one in its last week says when it expires, with Promote beside
  // it; an expired one is dimmed, until it's cleared.
  const expiry = a.status === "live" ? expiryOf(a, Date.now()) : undefined;
  const dimmed = a.status === "discarded" || !!expiry?.expired;
  return (
    <li id={`artifact-${a.id}`} role="option" aria-selected={selected} className={`row artifact-list-row${selected ? " selected" : ""}${dimmed ? " dimmed" : ""}`} onMouseMove={onHover}>
      <TLink to={artifactHref(a.id)} className="row-link">
        <Status s={artifactStatus(a)} iconOnly size={16} className="row-status" />
        <span className="row-main">
          <span className="row-line">
            <span className="title">{a.title}</span>
            <span className="chip tone-quiet" title={`${a.versions} ${a.versions === 1 ? "version" : "versions"}`}>
              v{v?.n ?? a.versions}
            </span>
            {a.status !== "live" || a.gone || inFlight(a) || a.expired ? <span className={`chip tone-${artifactStatus(a).tone}`}>{artifactStatus(a).words}</span> : null}
            {expiry?.soon && (
              <span className="chip tone-attention" title={`No new version since ${new Date(v?.at ?? a.made).toLocaleDateString()}: promote it to keep it`}>
                <Icon name="clock" size={11} /> {expiry.words}
              </span>
            )}
          </span>
          {v?.note && (
            <span className="row-sub">
              <Inline text={v.note} lines={1} />
            </span>
          )}
          <span className="row-sub artifact-row-meta">
            {!!a.from?.length && (
              <span>
                from{" "}
                {a.from.map((f, i) => (
                  <span key={f.from}>
                    {i > 0 && ", "}
                    <From from={f.from} title={f.title} workspace={workspace} inline />
                  </span>
                ))}
              </span>
            )}
            <span title={keptTitle(a)}>
              <Icon name={keptIcon(a.place)} size={11} /> {a.inFlight ?? keptWords(a.place)}
            </span>
            <span className="mono">{a.id}</span>
          </span>
        </span>
        {v && (
          <span className="age artifact-row-when" title={`v${v.n} ${byWords(v)}`}>
            {byWords(v)} · <Time iso={v.at} as="age" />
          </span>
        )}
      </TLink>
      {(expiry?.soon || expiry?.expired) && (
        <button className="ghost small artifact-row-promote" onClick={onPromote} title="Keep it for good: it expires no more">
          <Icon name="check-circle" size={13} /> Promote
        </button>
      )}
    </li>
  );
}

// --- An artifact

// ArtifactPage is an artifact, with a version running in a frame that
// fills the page: /artifacts/a3 (its latest), /artifacts/a3/v2.
export function ArtifactPage({ workspace }: { workspace: Workspace }) {
  const { id = "", version } = useParams();
  const navigate = useNavigate();
  const brief = workspace.artifacts?.find((a) => a.id === id);
  // Read again when the workspace says it's changed: a new version, or
  // where it stands.
  const key = brief ? `${brief.versions}:${brief.status}:${brief.latest?.at ?? ""}:${brief.title}:${brief.place}:${brief.promoted?.version ?? ""}:${brief.promoted?.into ?? ""}` : "";
  const { value: d, error, reload } = useLoaded<ArtifactDetail>(() => getArtifactDetail(id), [id, key], { keep: true });
  const asked = versionOf(version);
  const n = asked || d?.latest?.n || 0;
  const v = d?.all.find((x) => x.n === n);
  useTitle(d ? `${d.title} · v${n}` : id);

  const [width, setWidth] = useState(widths[0].key);
  const [run, setRun] = useState(0); // reload: a new frame
  const [stopped, setStopped] = useState(false);
  const [source, setSource] = useState(false);
  const [dialog, setDialog] = useState<"" | "promote" | "discard">("");
  // A version asked for runs: stepping to another starts it again.
  useEffect(() => setStopped(false), [id, n]);

  const go = (to: number) => d && to !== n && navigate(artifactHref(id, to), { replace: true });
  // What Compare opens on: this version beside another, or beside another
  // artifact (the newest), when it has only the one.
  const other = (workspace.artifacts ?? []).find((a) => a.id !== id && a.status !== "discarded" && a.latest);
  const comparing = d && v?.size && !d.gone ? compareFrom(id, v.n, d.all, other?.latest && { id: other.id, n: other.latest.n }) : undefined;
  useKeys({
    "[": () => d && go(stepVersion(d.all, n, -1)),
    "]": () => d && go(stepVersion(d.all, n, 1)),
    w: () => setWidth((w) => widths[(widths.findIndex((x) => x.key === w) + 1) % widths.length].key),
    r: () => {
      setStopped(false);
      setRun((x) => x + 1);
    },
    x: () => setStopped((s) => !s),
    s: () => setSource((s) => !s),
    c: () => comparing && navigate(compareHref(comparing)),
    p: () => d && d.status !== "discarded" && !d.gone && !d.inFlight && setDialog("promote"),
    O: () => n && window.open(frameURL(id, n), "_blank", "noopener,noreferrer"),
    Escape: () => navigate("/artifacts"),
  });

  if (error && !d) {
    return (
      <div className="page narrow">
        <ErrorCard error={error} retry={reload} title={`Couldn't open ${id}`} />
        <p>
          <Link to="/artifacts">Back to Artifacts</Link>
        </p>
      </div>
    );
  }
  if (!d) return <Loading what={`Opening ${id}…`} />;
  const st = artifactStatus(d);
  const px = widths.find((w) => w.key === width)?.px ?? 0;
  const missing = !!asked && !v;
  const runnable = !d.gone && !!v?.size;
  const expiry = d.status === "live" ? expiryOf(d, Date.now()) : undefined;
  const keepable = d.status !== "discarded" && !d.gone && !d.inFlight;

  return (
    <div className="artifact-view">
      <header className="artifact-head">
        <div className="artifact-title-line">
          <Link to="/artifacts" className="ghost icon-button small" aria-label="Back to Artifacts" title="Back to Artifacts">
            <Icon name="arrow-left" size={14} />
          </Link>
          <h1>{d.title}</h1>
          <span className={`chip tone-${st.tone}`}>
            <Icon name={st.icon} size={11} />
            {st.words}
          </span>
          <Copyable text={v ? `${d.id}.v${v.n}` : d.id} className="ref" title="Copy its ref" />
        </div>
        {v && <VersionLine d={d} v={v} workspace={workspace} />}
        {v?.note && (
          <div className="artifact-note">
            <Inline text={v.note} />
          </div>
        )}
        {d.status === "discarded" && d.discarded && (
          <div className="artifact-note dim">
            Discarded <Time iso={d.discarded.at} as="relative" /> by {d.discarded.by}
            {d.discarded.reason ? `: ${d.discarded.reason}${/[.!?]$/.test(d.discarded.reason) ? "" : "."}` : "."} It's off the list, and takes no new versions.{" "}
            {d.erased
              ? "Its files have been deleted for good."
              : d.place !== ".ws" && d.discarded.commit
                ? `History keeps it, at ${d.place}@${d.discarded.commit.slice(0, 7)}.`
                : d.place === ".ws"
                  ? `Its folder is kept in .ws/artifacts/.discarded until ${new Date(Date.parse(d.discarded.at) + KEEP_DISCARDED).toLocaleDateString()}.`
                  : null}
          </div>
        )}
        {d.status === "promoted" && d.promoted && (
          <div className="artifact-note dim">
            <Icon name="check-circle" size={12} /> Promoted <Time iso={d.promoted.at} as="relative" /> by {d.promoted.by}: v{d.promoted.version} was chosen
            {d.promoted.into ? (
              <>
                , into{" "}
                {workspace.missions.some((m) => m.slug === d.promoted?.into) ? (
                  <RefTag refKey={refKey({ kind: "mission", slug: d.promoted.into })} inline>
                    {d.promoted.into}
                  </RefTag>
                ) : (
                  <span className="mono">{d.promoted.into}</span>
                )}
              </>
            ) : null}
            . It's kept for good.
          </div>
        )}
        {expiry && (expiry.soon || expiry.expired) && (
          <div className={`artifact-note artifact-expiry${expiry.expired ? " expired" : ""}`}>
            <Icon name="clock" size={12} />{" "}
            {expiry.expired
              ? d.place === ".ws"
                ? "Expired: it's had no new version for a while, and the daemon discards it today. Promote it to keep it."
                : `Expired: it's had no new version for a while. Clear expired, on Artifacts, takes it from ${d.place}; promote it to keep it.`
              : `It ${expiry.words}: a new version, or promoting it, keeps it.`}
          </div>
        )}
        <div className="artifact-tools">
          <VersionStrip d={d} n={n} go={go} />
          <div className="segmented" role="group" aria-label="Width">
            {widths.map((w) => (
              <button key={w.key} aria-pressed={width === w.key} onClick={() => setWidth(w.key)} title={w.title} disabled={source}>
                {w.label}
              </button>
            ))}
          </div>
          <span className="artifact-tools-end">
            <button
              className="ghost small"
              onClick={() => {
                setStopped(false);
                setRun((x) => x + 1);
              }}
              disabled={!runnable || source}
              title="Run it again"
            >
              <Icon name="rotate" size={13} /> Reload <Keys k="r" />
            </button>
            <button className="ghost small" onClick={() => setStopped((s) => !s)} disabled={!runnable || source} title={stopped ? "Run it again" : "Unload it: a script that loops stops"}>
              <Icon name={stopped ? "play" : "stop"} size={13} /> {stopped ? "Run" : "Stop"} <Keys k="x" />
            </button>
            <button className="ghost small" aria-pressed={source} onClick={() => setSource((s) => !s)} disabled={!runnable} title="Its HTML">
              <Icon name="code" size={13} /> Source <Keys k="s" />
            </button>
            {comparing ? (
              <Link
                className="button-link ghost small"
                to={compareHref(comparing)}
                title={comparing[0].id === d.id ? `v${comparing[0].n} and v${comparing[1].n}, side by side: two frames, or their source diff` : `Beside ${comparing[0].id}, side by side: two frames, or their source diff`}
              >
                <Icon name="split" size={13} /> Compare <Keys k="c" />
              </Link>
            ) : runnable ? (
              <button className="ghost small" disabled title="It has one version, and there's no other artifact to compare it with">
                <Icon name="split" size={13} /> Compare
              </button>
            ) : null}
            {runnable ? (
              <a className="button-link ghost small" href={frameURL(d.id, n)} target="_blank" rel="noopener noreferrer" title="In a tab of its own, still sandboxed">
                <Icon name="external" size={13} /> Open alone
              </a>
            ) : null}
            {keepable && (
              <button className="ghost small" onClick={() => setDialog("promote")} title={d.status === "promoted" ? "Choose another version, or say what it went into now" : "Keep it for good: it expires no more"}>
                <Icon name="check-circle" size={13} /> {d.status === "promoted" ? "Promoted" : "Promote"} <Keys k="p" />
              </button>
            )}
            {keepable && (
              <button className="ghost small" onClick={() => setDialog("discard")} title="Take it off the list, saying why: it can be restored">
                <Icon name="ban" size={13} /> Discard
              </button>
            )}
            {d.status === "discarded" && d.restorable && (
              <button className="ghost small" onClick={() => restore(d, workspace).then((done) => done && reload())} title="Put it back, as it was">
                <Icon name="rotate" size={13} /> Restore
              </button>
            )}
          </span>
        </div>
      </header>
      <div className="artifact-stage">
        {missing ? (
          <Empty icon="search" title={`${d.id} has no v${asked}`} action={<Link to={artifactHref(d.id)}>Its latest, v{d.latest?.n}</Link>}>
            Its versions are {d.all.map((x) => `v${x.n}`).join(", ")}.
          </Empty>
        ) : !runnable ? (
          <Empty icon="alert" title="There's no file to run">
            {d.gone ? `Its folder has gone from ${d.where.replace(/[^/]+\/$/, "")}: what's here is what the records kept.` : `v${n}'s file isn't in ${d.where}.`}
          </Empty>
        ) : source ? (
          <Source id={d.id} n={n} />
        ) : stopped ? (
          <Empty icon="stop" title="Stopped" action={<button onClick={() => setStopped(false)}>Run it</button>}>
            The frame's unloaded, and its scripts with it.
          </Empty>
        ) : (
          <div className={`artifact-frame-box${px ? " sized" : ""}`}>
            <iframe
              key={`${d.id}.v${n}#${run}`}
              className="artifact-frame"
              src={frameURL(d.id, n)}
              sandbox="allow-scripts"
              referrerPolicy="no-referrer"
              title={`${d.title}, v${n}`}
              style={px ? { width: px } : undefined}
            />
          </div>
        )}
      </div>
      {dialog === "promote" && <PromoteDialog a={d} version={v?.n} workspace={workspace} onClose={() => setDialog("")} onDone={() => reload()} />}
      {dialog === "discard" && <DiscardDialog a={d} workspace={workspace} onClose={() => setDialog("")} onDone={() => reload()} />}
    </div>
  );
}

// VersionLine is a version in a line: v2 of 3, based on v1 (when it isn't
// the one before), from where, by whom, and when.
export function VersionLine({ d, v, workspace }: { d: ArtifactDetail; v: ArtifactVersion; workspace: Workspace }) {
  return (
    <div className="artifact-version-line">
      <span>
        v{v.n} of {d.all.length}
      </span>
      {v.basedOn && v.basedOn !== v.n - 1 ? <span>based on v{v.basedOn}</span> : null}
      {v.from && (
        <span>
          from <From from={v.from} title={v.fromTitle} workspace={workspace} />
          {v.proposal ? `, proposal ${v.proposal}` : ""}
        </span>
      )}
      <span>{v.from ? byWords(v) : v.by === "you" ? "by you, by hand" : byWords(v)}</span>
      <Time iso={v.at} as="relative" />
      {v.size ? <span>{sizeWords(v.size)}</span> : null}
      {v.capsule ? (
        <span className="artifact-in-flight" title={v.was ? `v${v.was} in ${v.capsule}: v${v.was} landed first, so it lands as v${v.n}` : `In ${v.capsule}: it lands with it`}>
          <Icon name="circle-dashed" size={11} /> in flight, in {v.capsule}
          {v.was ? `, as v${v.was}` : ""}
        </span>
      ) : (
        <span title={keptTitle(d)}>
          <Icon name={keptIcon(d.place)} size={11} /> {keptWords(d.place)}
        </span>
      )}
    </div>
  );
}

// VersionStrip is the versions, v1 v2 v3, each with its note on hover,
// and the keys that step through them.
export function VersionStrip({ d, n, go, keys = ["[", "]"] }: { d: ArtifactDetail; n: number; go: (n: number) => void; keys?: [string, string] }) {
  return (
    <div className="segmented artifact-versions" role="group" aria-label="Versions">
      {d.all.map((x) => (
        <HoverCard
          key={x.n}
          className="artifact-version-host"
          trigger={
            <button aria-pressed={x.n === n} onClick={() => go(x.n)} tabIndex={-1} className={x.capsule ? "in-flight" : undefined}>
              v{x.n}
            </button>
          }
        >
          <span className="hovercard-head">
            <span className="hovercard-title">v{x.n}</span>
            <span className="dim">
              {byWords(x)}, <Time iso={x.at} as="relative" />
            </span>
          </span>
          <span className="hovercard-line">{x.note || <span className="dim">No note.</span>}</span>
          {x.capsule && <span className="hovercard-line dim">In flight, in {x.capsule}: it lands with it.</span>}
        </HoverCard>
      ))}
      <span className="artifact-step-keys dim">
        <Keys k={keys[0]} /> <Keys k={keys[1]} />
      </span>
    </div>
  );
}

// Source is a version's HTML, as Files shows a file.
function Source({ id, n }: { id: string; n: number }) {
  const { value, error } = useLoaded(() => getArtifactSource(id, n), [id, n]);
  const ready = useHighlighter();
  const lines = useMemo(() => {
    if (!value) return [];
    const ls = value.text.split("\n");
    return ready && ls.length <= 20000 ? highlightLines(ls, languageOf("v.html")) : ls.map(escapeHTML);
  }, [value, ready]);
  if (error) return <ErrorCard error={error} />;
  if (!value) return <Loading what="Reading its source…" />;
  return (
    <div className="code-view artifact-source">
      {lines.map((h, i) => (
        <div key={i} className="code-line">
          <span className="code-ln">{i + 1}</span>
          <span className="code-mark" aria-hidden />
          <code dangerouslySetInnerHTML={{ __html: h || " " }} />
        </div>
      ))}
    </div>
  );
}
