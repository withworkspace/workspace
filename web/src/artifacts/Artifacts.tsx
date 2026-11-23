import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router";
import { type ArtifactDetail, type ArtifactInfo, type ArtifactSource, type ArtifactVersion, frameURL, getArtifactDetail, getArtifactSource, promoteArtifact } from "../api";
import { useKeys } from "../keys";
import { useLoaded } from "../load";
import { RefTag } from "../refs/RefTag";
import { refKey, refName } from "../refs/refs";
import { escapeHTML, highlightLines, languageOf, useHighlighter } from "../review/highlight";
import { HoverCard, HoverCardHead, HoverCardLine, HoverCardTitle } from "../ui/HoverCard";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/inline";
import { toast, useTitle } from "../ui/kit";
import { Loading } from "../ui/Loading";
import { Empty } from "../ui/Empty";
import { humanError } from "../ui/text";
import { ErrorCard } from "../ui/ErrorCard";
import { Copyable } from "../ui/Copyable";
import { Keys } from "../ui/Keys";
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
  filesWords,
  KEEP_DISCARDED,
  fromRef,
  isConversation,
  keptWords,
  keptIcon,
  keptTitle,
  behind,
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
import { hasTweaks, TweaksToggle, TweaksTray, useTweaks } from "./Tray";
import { changed, followed, frameSearch, readTweakParams, TWEAK, tweakParams } from "./tweaks";
import { ClearExpired, DiscardDialog, PromoteDialog, restore } from "./Lifetime";
import { PagePin } from "../shell/focus";
import { FoundArtifacts } from "./Found";
import { Chip } from "../ui/Chip";
import { Button, buttonClass } from "../ui/Button";
import { PageHead } from "../ui/PageHead";
import { Row, RowLine, RowMain, RowSub, RowTitle, Rows, rowLinkClass, rowStatusClass } from "../ui/Rows";
import { Segment, Segmented, segmentClass, segmentedClass } from "../ui/Segmented";
import { ToggleButton } from "../ui/ToggleButton";
import { useWorkspace } from "../workspace/store";
import { useThingMenu } from "../shell/ThingMenu";

// Artifacts: versioned HTML prototypes, listed, and each run in a
// sandboxed frame. A version is its page and the files it loads beside
// it. It runs from a route of its own (/frame/a3/v2, which sends the
// frame to where the version's files are), in <iframe
// sandbox="allow-scripts">: its scripts run, in an origin of their own,
// so they can't read ws's cookie, its storage or this page, and its
// policy lets it load its own files and nothing else. What its page says can be changed
// about it (its tweaks) is drawn beside the frame (Tray.tsx), and the
// state they're in is in the address.

export const artifactsKeys: [string, string][] = [
  ["j / k", "Move down and up the list"],
  ["⏎ or o", "Open the artifact"],
];

export const artifactKeys: [string, string][] = [
  ["[ / ]", "The version before, or after"],
  ["w", "The next width: fit, 1280, 768, 390"],
  ["r", "Reload: run it again"],
  ["x", "Stop it, or run it again"],
  ["s", "Its source, file by file, or the frame again"],
  ["t", "Its tweaks, shown or hidden: what its page says can be changed"],
  ["c", "Compare it, side by side: beside the version it's based on"],
  ["p", "Promote it: keep it for good, choosing this version"],
  ["O", "Open it alone, in a tab of its own"],
  ["esc", "Back to Artifacts"],
];

export const artifactHref = (id: string, n?: number) => (n ? `/artifacts/${id}/v${n}` : `/artifacts/${id}`);

// From is where a version came from: a conversation or a mission, drawn
// as its ref where this machine has it, its title where it hasn't (one
// made on another machine).
function From({ from, title, inline }: { from: string; title?: string; inline?: boolean }) {
  const talk = missionTalk(from);
  const here = useWorkspace((w) => (isConversation(from) ? w.conversations.some((t) => t.id === from) : w.missions.some((m) => m.slug === (talk?.slug ?? from))));
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

export function ArtifactsPage() {
  const artifacts = useWorkspace((w) => w.artifacts);
  useTitle("Artifacts");
  const navigate = useNavigate();
  const all = useMemo(() => artifacts ?? [], [artifacts]);
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
      <PageHead
        title="Artifacts"
        actions={
          all.length > 0 && (
            <>
              <Segmented className="artifacts-shown" value={shown} onChange={setShown} aria-label="Where they stand">
                {(Object.keys(shownLabels) as Shown[]).map((s) => (
                  <Segment key={s} value={s}>
                    {shownLabels[s]}
                    {s === "discarded" && discarded > 0 ? ` ${discarded}` : ""}
                  </Segment>
                ))}
              </Segmented>
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
              <ClearExpired all={all} />
            </>
          )
        }
      />
      {all.length === 0 ? (
        <Empty icon="artifact" title="No artifacts yet">
          An artifact is a sketch to look at and use: a page, alone or with the files it loads beside it, in versions you can step through. Crew add them with{" "}
          <code>ws artifact add</code>, and so can you.
        </Empty>
      ) : list.length === 0 ? (
        <Empty icon="search" title="None here">
          No artifact {shown === "all" ? "" : `that's ${shownLabels[shown].toLowerCase()} `}came from there.
        </Empty>
      ) : (
        <Rows className="artifacts" role="listbox" aria-label="Artifacts">
          {list.map((a, i) => (
            <ArtifactListRow key={a.id} a={a} selected={i === index} onHover={() => i !== index && setIndex(i)} onPromote={() => setPromoting(a)} />
          ))}
        </Rows>
      )}
      <FoundArtifacts />
      {promoting && <PromoteDialog a={promoting} onClose={() => setPromoting(undefined)} />}
    </div>
  );
}

function ArtifactListRow({ a, selected, onHover, onPromote }: { a: ArtifactInfo; selected: boolean; onHover: () => void; onPromote: () => void }) {
  const menu = useThingMenu();
  const v = a.latest;
  // A live one in its last week says when it expires, with Promote beside
  // it; an expired one is dimmed, until it's cleared.
  const expiry = a.status === "live" ? expiryOf(a, Date.now()) : undefined;
  const dimmed = a.status === "discarded" || !!expiry?.expired;
  return (
    <Row selected={selected} id={`artifact-${a.id}`} role="option" aria-selected={selected} className={`artifact-list-row${dimmed ? " dimmed" : ""}`} onMouseMove={onHover}>
      <TLink to={artifactHref(a.id)} className={rowLinkClass("artifact-list-link")} onContextMenu={menu(artifactHref(a.id), a.title)}>
        <Status s={artifactStatus(a)} iconOnly size={16} className={rowStatusClass()} />
        <RowMain>
          <RowLine>
            <RowTitle className="artifact-list-title">{a.title}</RowTitle>
            <Chip tone="quiet" title={`${a.versions} ${a.versions === 1 ? "version" : "versions"}`}>
              v{v?.n ?? a.versions}
            </Chip>
            {a.status !== "live" || a.gone || a.expired ? <Chip tone={artifactStatus(a).tone}>{artifactStatus(a).words}</Chip> : null}
            {expiry?.soon && (
              <Chip tone="attention" title={`No new version since ${new Date(v?.at ?? a.made).toLocaleDateString()}: promote it to keep it`}>
                <Icon name="clock" size={11} /> {expiry.words}
              </Chip>
            )}
          </RowLine>
          {v?.note && (
            <RowSub>
              <Inline text={v.note} lines={1} />
            </RowSub>
          )}
          <RowSub className="artifact-row-meta">
            {!!a.from?.length && (
              <span>
                from{" "}
                {a.from.map((f, i) => (
                  <span key={f.from}>
                    {i > 0 && ", "}
                    <From from={f.from} title={f.title} inline />
                  </span>
                ))}
              </span>
            )}
            <span title={keptTitle(a)}>
              <Icon name={keptIcon(a)} size={11} /> {keptWords(a)}
              {behind(a) > 0 && <span className="dim">, v{a.latest?.n} here</span>}
            </span>
            <span className="mono">{a.id}</span>
          </RowSub>
        </RowMain>
        {v && (
          <span className="age artifact-row-when" title={`v${v.n} ${byWords(v)}`}>
            {byWords(v)} · <Time iso={v.at} as="age" />
          </span>
        )}
      </TLink>
      {(expiry?.soon || expiry?.expired) && (
        <Button variant="ghost" size="small" className="artifact-row-promote" onClick={onPromote} title="Keep it for good: it expires no more">
          <Icon name="check-circle" size={13} /> Promote
        </Button>
      )}
    </Row>
  );
}

// --- An artifact

// ArtifactPage is an artifact, with a version running in a frame that
// fills the page: /artifacts/a3 (its latest), /artifacts/a3/v2.
export function ArtifactPage() {
  const artifacts = useWorkspace((w) => w.artifacts);
  const missions = useWorkspace((w) => w.missions);
  const { id = "", version } = useParams();
  const navigate = useNavigate();
  const brief = artifacts?.find((a) => a.id === id);
  // Read again when the workspace says it's changed: a new version, or
  // where it stands.
  const key = brief ? `${brief.versions}:${brief.status}:${brief.latest?.at ?? ""}:${brief.title}:${brief.home?.where ?? ""}:${brief.home?.version ?? ""}:${brief.promoted?.version ?? ""}:${brief.promoted?.into ?? ""}` : "";
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

  // Its tweaks: what the frame's page declares, drawn in a tray beside it.
  // The address says the state where it isn't the one the page opens as,
  // and follows it as it changes. A frame opens in the state the address
  // gives when it's made (a version stepped to, a reload), and is told of
  // changes after that: its own address stays as it was, or every change
  // would load it again.
  const [search] = useSearchParams();
  const tweaks = useTweaks(`${id}.v${n}#${run}`);
  const [tray, setTray] = useState(true);
  const told = tweaks.told;
  useEffect(() => {
    const next = followed(search, [[TWEAK, told]]);
    if (next !== undefined) navigate({ search: next }, { replace: true });
  }, [told, search, navigate]);
  const opensAs = frameSearch(readTweakParams(search));
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const frameSrc = useMemo(() => frameURL(id, n, opensAs), [id, n, run, stopped, source]);

  const go = (to: number) => d && to !== n && navigate(artifactHref(id, to), { replace: true });
  // What Compare opens on: this version beside another, or beside another
  // artifact (the newest), when it has only the one.
  const other = (artifacts ?? []).find((a) => a.id !== id && a.status !== "discarded" && a.latest);
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
    t: () => hasTweaks(told) && setTray((x) => !x),
    c: () => comparing && navigate(compareHref(comparing)),
    p: () => d && d.status !== "discarded" && !d.gone && setDialog("promote"),
    O: () => n && window.open(frameURL(id, n, opensAs), "_blank", "noopener,noreferrer"),
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
  const keepable = d.status !== "discarded" && !d.gone;
  // The frame's on the page: there's a version, running, not its source.
  const framed = !missing && runnable && !source && !stopped;

  return (
    <div className="artifact-view">
      <header className="artifact-head">
        <div className="artifact-title-line">
          <Link to="/artifacts" className="ghost icon-button small" aria-label="Back to Artifacts" title="Back to Artifacts">
            <Icon name="arrow-left" size={14} />
          </Link>
          <h1>{d.title}</h1>
          <Chip tone={st.tone}>
            <Icon name={st.icon} size={11} />
            {st.words}
          </Chip>
          <Copyable text={v ? `${d.id}.v${v.n}` : d.id} className="artifact-title-ref" title="Copy its ref" />
          <PagePin r={{ kind: "artifact", id: d.id }} />
        </div>
        {v && <VersionLine d={d} v={v} />}
        {v?.note && (
          <div className="artifact-note">
            <Inline text={v.note} />
          </div>
        )}
        {d.status === "discarded" && d.discarded && (
          <div className="artifact-note dim">
            Discarded <Time iso={d.discarded.at} as="relative" /> by {d.discarded.by}
            {d.discarded.reason ? `: ${d.discarded.reason}${/[.!?]$/.test(d.discarded.reason) ? "" : "."}` : "."} It's off the list, and takes no new versions.{" "}
            {d.erased ? "Its files have been deleted for good." : `Its folder is kept in .ws/artifacts/.discarded until ${new Date(Date.parse(d.discarded.at) + KEEP_DISCARDED).toLocaleDateString()}.`}
          </div>
        )}
        {d.status === "promoted" && d.promoted && (
          <div className="artifact-note dim">
            <Icon name="check-circle" size={12} /> Promoted <Time iso={d.promoted.at} as="relative" /> by {d.promoted.by}: v{d.promoted.version} was chosen
            {d.promoted.into ? (
              <>
                , into{" "}
                {missions.some((m) => m.slug === d.promoted?.into) ? (
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
        {d.home && d.status === "promoted" && <HomeLine d={d} n={n} onDone={reload} />}
        {expiry && (expiry.soon || expiry.expired) && (
          <div className={`artifact-note artifact-expiry${expiry.expired ? " expired" : ""}`}>
            <Icon name="clock" size={12} />{" "}
            {expiry.expired ? "Expired: it's had no new version for a while, and the daemon discards it today. Promote it to keep it." : `It ${expiry.words}: a new version, or promoting it, keeps it.`}
          </div>
        )}
        <div className="artifact-tools">
          <VersionStrip d={d} n={n} go={go} />
          <Segmented value={width} onChange={setWidth} disabled={source} aria-label="Width">
            {widths.map((w) => (
              <Segment key={w.key} value={w.key} title={w.title}>
                {w.label}
              </Segment>
            ))}
          </Segmented>
          <span className="artifact-tools-end">
            <Button
              variant="ghost" size="small"
              onClick={() => {
                setStopped(false);
                setRun((x) => x + 1);
              }}
              disabled={!runnable || source}
              title="Run it again"
            >
              <Icon name="rotate" size={13} /> Reload <Keys k="r" />
            </Button>
            <Button variant="ghost" size="small" onClick={() => setStopped((s) => !s)} disabled={!runnable || source} title={stopped ? "Run it again" : "Unload it: a script that loops stops"}>
              <Icon name={stopped ? "play" : "stop"} size={13} /> {stopped ? "Run" : "Stop"} <Keys k="x" />
            </Button>
            <ToggleButton variant="ghost" size="small" pressed={source} onChange={setSource} disabled={!runnable} title={v?.files && v.files > 1 ? "Its files, as they were written" : "Its HTML"}>
              <Icon name="code" size={13} /> Source <Keys k="s" />
            </ToggleButton>
            {framed && <TweaksToggle told={told} shown={tray} onToggle={() => setTray((x) => !x)} />}
            {comparing ? (
              <Link
                className={buttonClass({ variant: "ghost" }, "small")}
                to={compareHref(comparing)}
                title={comparing[0].id === d.id ? `v${comparing[0].n} and v${comparing[1].n}, side by side: two frames, or their source diff` : `Beside ${comparing[0].id}, side by side: two frames, or their source diff`}
              >
                <Icon name="split" size={13} /> Compare <Keys k="c" />
              </Link>
            ) : runnable ? (
              <Button variant="ghost" size="small" unavailable="It has one version, and there's no other artifact to compare it with">
                <Icon name="split" size={13} /> Compare
              </Button>
            ) : null}
            {runnable ? (
              <a className={buttonClass({ variant: "ghost" }, "small")} href={frameURL(d.id, n, opensAs)} target="_blank" rel="noopener noreferrer" title="In a tab of its own, still sandboxed, as it's set here">
                <Icon name="external" size={13} /> Open alone
              </a>
            ) : null}
            {keepable && (
              <Button variant="ghost" size="small" onClick={() => setDialog("promote")} title={d.status === "promoted" ? "Choose another version, or say what it went into now" : "Keep it for good: it expires no more"}>
                <Icon name="check-circle" size={13} /> {d.status === "promoted" ? "Promoted" : "Promote"} <Keys k="p" />
              </Button>
            )}
            {keepable && (
              <Button variant="ghost" size="small" onClick={() => setDialog("discard")} title="Take it off the list, saying why: it can be restored">
                <Icon name="ban" size={13} /> Discard
              </Button>
            )}
            {d.status === "discarded" && d.restorable && (
              <Button variant="ghost" size="small" onClick={() => restore(d).then((done) => done && reload())} title="Put it back, as it was">
                <Icon name="rotate" size={13} /> Restore
              </Button>
            )}
          </span>
        </div>
      </header>
      <div className="artifact-body">
        <div className="artifact-stage">
          {missing ? (
            <Empty className="artifact-empty" icon="search" title={`${d.id} has no v${asked}`} action={<Link to={artifactHref(d.id)}>Its latest, v{d.latest?.n}</Link>}>
              Its versions are {d.all.map((x) => `v${x.n}`).join(", ")}.
            </Empty>
          ) : !runnable ? (
            <Empty className="artifact-empty" icon="alert" title="There's no file to run">
              {d.gone ? `Its folder has gone from ${d.where.replace(/[^/]+\/$/, "")}: what's here is what the records kept.` : `v${n}'s files aren't in ${d.where}.`}
            </Empty>
          ) : source ? (
            <Source id={d.id} n={n} />
          ) : stopped ? (
            <Empty className="artifact-empty" icon="stop" title="Stopped" action={<Button onClick={() => setStopped(false)}>Run it</Button>}>
              The frame's unloaded, and its scripts with it.
            </Empty>
          ) : (
            <div className={`artifact-frame-box${px ? " sized" : ""}`}>
              <iframe
                key={`${d.id}.v${n}#${run}`}
                ref={tweaks.frame}
                className="artifact-frame"
                src={frameSrc}
                sandbox="allow-scripts"
                referrerPolicy="no-referrer"
                title={`${d.title}, v${n}`}
                style={px ? { width: px } : undefined}
              />
            </div>
          )}
        </div>
        {framed && tray && hasTweaks(told) && (
          <TweaksTray told={told} set={tweaks.set} onClose={() => setTray(false)}>
            {changed(told).length > 0 && (
              <Link className="link" to={compareHref([{ id: d.id, n }, { id: d.id, n }], [[], tweakParams(told)])} title="The same version twice, side by side: as it opens, and as it's set here">
                <Icon name="split" size={12} /> Compare with how it opens
              </Link>
            )}
          </TweaksTray>
        )}
      </div>
      {dialog === "promote" && <PromoteDialog a={d} version={v?.n} onClose={() => setDialog("")} onDone={() => reload()} />}
      {dialog === "discard" && <DiscardDialog a={d} onClose={() => setDialog("")} onDone={() => reload()} />}
    </div>
  );
}

// HomeLine says where a promoted artifact lives in a repo, and which
// version is there. Where the version on the page is another, it's one
// press to put it there: where it lives was chosen the first time, so
// nothing's asked.
function HomeLine({ d, n, onDone }: { d: ArtifactDetail; n: number; onDone: () => void }) {
  const [busy, setBusy] = useState(false);
  const home = d.home;
  if (!home) return null;
  const put = async () => {
    if (busy) return;
    setBusy(true);
    try {
      const done = await promoteArtifact(d.id, { version: n, into: d.promoted?.into || undefined });
      toast(`Promoted ${d.id}: v${n} is in ${home.where}${done.landed ? ` (${done.landed})` : ""}`, { tone: "done", ms: 6000 });
      onDone();
    } catch (e) {
      toast(humanError((e as Error).message), { tone: "danger", ms: 8000 });
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="artifact-note artifact-home dim">
      <Icon name="branch" size={12} /> It lives in <Copyable text={home.where} className="artifact-home-where" title="Copy its folder: how it's named for other people" /> too, for everyone with {home.repo}: v{home.version} is there
      {home.commit ? ` (${home.commit.slice(0, 7)})` : ""}.
      {n && n !== home.version && d.all.some((x) => x.n === n) ? (
        <Button variant="ghost" size="small" className="artifact-home-action" onClick={put} disabled={busy} title={`Promote v${n}: it takes v${home.version}'s place there, in one commit`}>
          <Icon name="check-circle" size={12} /> {busy ? "Landing…" : `Put v${n} there`}
        </Button>
      ) : null}
    </div>
  );
}

// VersionLine is a version in a line: v2 of 3, based on v1 (when it isn't
// the one before), from where, by whom, and when.
export function VersionLine({ d, v }: { d: ArtifactDetail; v: ArtifactVersion }) {
  return (
    <div className="artifact-version-line">
      <span>
        v{v.n} of {d.all.length}
      </span>
      {v.basedOn && v.basedOn !== v.n - 1 ? <span>based on v{v.basedOn}</span> : null}
      {v.from && (
        <span>
          from <From from={v.from} title={v.fromTitle} />
          {v.proposal ? `, proposal ${v.proposal}` : ""}
        </span>
      )}
      <span>{v.from ? byWords(v) : v.by === "you" ? "by you, by hand" : byWords(v)}</span>
      <Time iso={v.at} as="relative" />
      {v.size ? <span>{filesWords(v.files, v.size)}</span> : null}
      {d.home?.version === v.n ? (
        <span title={keptTitle(d)}>
          <Icon name="branch" size={11} /> in {d.home.repo}
        </span>
      ) : (
        <span title={`On this machine: ${d.where}`}>
          <Icon name="monitor" size={11} /> This machine
        </span>
      )}
    </div>
  );
}

// VersionStrip is the versions, v1 v2 v3, each with its note on hover,
// and the keys that step through them.
export function VersionStrip({ d, n, go, keys = ["[", "]"] }: { d: ArtifactDetail; n: number; go: (n: number) => void; keys?: [string, string] }) {
  return (
    <div className={segmentedClass({}, "artifact-versions")} role="group" aria-label="Versions">
      {d.all.map((x) => (
        <HoverCard
          key={x.n}
          className="artifact-version-host"
          trigger={
            <button data-ui="" aria-pressed={x.n === n} data-selected={x.n === n || undefined} onClick={() => go(x.n)} tabIndex={-1} className={segmentClass(d.home?.version === x.n ? "at-home" : undefined)}>
              v{x.n}
            </button>
          }
        >
          <HoverCardHead>
            <HoverCardTitle>v{x.n}</HoverCardTitle>
            <span className="dim">
              {byWords(x)}, <Time iso={x.at} as="relative" />
            </span>
          </HoverCardHead>
          <HoverCardLine>{x.note || <span className="dim">No note.</span>}</HoverCardLine>
          {d.home?.version === x.n && <HoverCardLine dim>It's the version in {d.home.repo}: {d.home.where}</HoverCardLine>}
        </HoverCard>
      ))}
      <span className="artifact-step-keys dim">
        <Keys k={keys[0]} /> <Keys k={keys[1]} />
      </span>
    </div>
  );
}

// Source is a version's files, as they were written: its page, and for
// one with more, a list of them beside it to read another. A file that
// isn't text (an image, a font) says what it is.
function Source({ id, n }: { id: string; n: number }) {
  const [file, setFile] = useState("");
  // Another version opens at its page.
  useEffect(() => setFile(""), [id, n]);
  const { value, error } = useLoaded<ArtifactSource>(() => getArtifactSource(id, n, file), [id, n, file], { keep: true });
  const ready = useHighlighter();
  const lines = useMemo(() => {
    if (!value || value.binary) return [];
    const ls = value.text.split("\n");
    return ready && ls.length <= 20000 ? highlightLines(ls, languageOf(value.file)) : ls.map(escapeHTML);
  }, [value, ready]);
  if (error && !value) return <ErrorCard error={error} />;
  if (!value) return <Loading what="Reading its source…" />;
  const many = value.files.length > 1;
  return (
    <div className={`artifact-source-view${many ? " many" : ""}`}>
      {many && (
        <nav className="artifact-files" aria-label={`v${value.version}'s files`}>
          <div className="artifact-files-head dim">
            {value.files.length} files, {sizeWords(value.files.reduce((sum, f) => sum + f.size, 0))}
          </div>
          {value.files.map((f) => (
            <button
              key={f.path}
              // The one being read is kept in sight: on a narrow window the list is a row that scrolls.
              ref={f.path === value.file ? (el) => el?.scrollIntoView({ block: "nearest", inline: "nearest" }) : undefined}
              className="artifact-file"
              aria-current={f.path === value.file ? "true" : undefined}
              onClick={() => setFile(f.path)}
              title={`${f.path}, ${sizeWords(f.size)}`}
            >
              <Icon name={f.path === "index.html" ? "artifact" : "file"} size={12} />
              <span className="artifact-file-path">{f.path}</span>
              <span className="artifact-file-size dim">{sizeWords(f.size)}</span>
            </button>
          ))}
        </nav>
      )}
      {error ? (
        <ErrorCard error={error} />
      ) : value.binary ? (
        <Empty className="artifact-empty" icon="file" title={`${value.file} isn't text`}>
          It's {sizeWords(value.size)}, and the page shows it where it uses it.
        </Empty>
      ) : (
        <div className="code-view artifact-source">
          {lines.map((h, i) => (
            <div key={i} className="code-line">
              <span className="code-ln">{i + 1}</span>
              <span className="code-mark" aria-hidden />
              <code dangerouslySetInnerHTML={{ __html: h || " " }} />
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
