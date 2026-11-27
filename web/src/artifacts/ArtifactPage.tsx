import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router";
import { type ArtifactDetail, frameURL, getArtifactDetail } from "../api";
import { useCommands } from "../keymap/useCommands";
import { artifactViewKeys } from "./keys";
import { useLoaded } from "../load";
import { RefTag } from "../refs/RefTag";
import { refKey } from "../refs/refs";
import { PagePin } from "../shell/PagePin";
import { Button } from "../ui/Button";
import { Chip } from "../ui/Chip";
import { ErrorCard } from "../ui/ErrorCard";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/Inline";
import { Loading } from "../ui/Loading";
import { Segment, Segmented } from "../ui/Segmented";
import { useTitle } from "../ui/useTitle";
import { useWorkspace } from "../workspace/store";
import { ArtifactActions } from "./ArtifactActions";
import { ArtifactBody, ArtifactStage } from "./ArtifactBody";
import { ArtifactDiscardedNote } from "./ArtifactDiscardedNote";
import { ArtifactEmpty } from "./ArtifactEmpty";
import { ArtifactExpiryNote } from "./ArtifactExpiryNote";
import { ArtifactFrame } from "./ArtifactFrame";
import { ArtifactHead } from "./ArtifactHead";
import { ArtifactNote } from "./ArtifactNote";
import { ArtifactPromotedNote } from "./ArtifactPromotedNote";
import { ArtifactTitleLine, ArtifactTitleRef } from "./ArtifactTitleLine";
import { ArtifactTools } from "./ArtifactTools";
import { ArtifactView } from "./ArtifactView";
import { DiscardDialog } from "./DiscardDialog";
import { HomeLine } from "./HomeLine";
import { artifactHref } from "./hrefs";
import { compareFrom, compareHref } from "./pair";
import { PromoteDialog } from "./PromoteDialog";
import { restore } from "./restore";
import { artifactStatus, expiryOf, KEEP_DISCARDED, stepVersion, versionOf, widths } from "./shown";
import { Source } from "./Source";
import { changed, followed, frameSearch, readTweakParams, TWEAK, tweakParams } from "./tweaks";
import { TweaksToggle } from "./TweaksToggle";
import { TweaksTray } from "./TweaksTray";
import { hasTweaks, useTweaks } from "./useTweaks";
import { VersionLine } from "./VersionLine";
import { VersionStrip } from "./VersionStrip";
import { Text, textClass } from "../ui/Text";

// An artifact's page. A version runs from a route of its own
// (/frame/a3/v2, which sends the frame to where the version's files are),
// in <iframe sandbox="allow-scripts"> (ArtifactFrame.tsx): its scripts
// run, in an origin of their own, so they can't read ws's cookie, its
// storage or this page, and its policy lets it load its own files and
// nothing else. What its page says can be changed about it (its tweaks) is
// drawn beside the frame (TweaksTray.tsx), and the state they're in is in
// the address.

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
  const again = () => {
    setStopped(false);
    setRun((x) => x + 1);
  };
  useCommands(artifactViewKeys, {
    prevVersion: () => d && go(stepVersion(d.all, n, -1)),
    nextVersion: () => d && go(stepVersion(d.all, n, 1)),
    width: () => setWidth((w) => widths[(widths.findIndex((x) => x.key === w) + 1) % widths.length].key),
    reload: again,
    stop: () => setStopped((s) => !s),
    source: () => setSource((s) => !s),
    tweaks: () => hasTweaks(told) && setTray((x) => !x),
    compare: () => comparing && navigate(compareHref(comparing)),
    promote: () => d && d.status !== "discarded" && !d.gone && setDialog("promote"),
    newTab: () => n && window.open(frameURL(id, n, opensAs), "_blank", "noopener,noreferrer"),
    back: () => navigate("/artifacts"),
  });

  if (error && !d) {
    return (
      <div className="page narrow">
        <ErrorCard error={error} retry={reload} title={`Could not open ${id}`} />
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
    <ArtifactView>
      <ArtifactHead>
        <ArtifactTitleLine back={{ to: "/artifacts", label: "Back to Artifacts", keys: artifactViewKeys.back }} title={d.title}>
          <Chip tone={st.tone}>
            <Icon name={st.icon} size={11} />
            {st.words}
          </Chip>
          <ArtifactTitleRef text={v ? `${d.id}.v${v.n}` : d.id} title="Copy ref" />
          <PagePin r={{ kind: "artifact", id: d.id }} end />
        </ArtifactTitleLine>
        {v && <VersionLine d={d} v={v} />}
        {v?.note && (
          <ArtifactNote>
            <Inline text={v.note} />
          </ArtifactNote>
        )}
        {d.status === "discarded" && d.discarded && (
          <ArtifactDiscardedNote at={d.discarded.at} by={d.discarded.by} reason={d.discarded.reason} erased={d.erased} until={new Date(Date.parse(d.discarded.at) + KEEP_DISCARDED).toLocaleDateString()} />
        )}
        {d.status === "promoted" && d.promoted && (
          <ArtifactPromotedNote
            at={d.promoted.at}
            by={d.promoted.by}
            version={d.promoted.version}
            into={
              d.promoted.into ? (
                missions.some((m) => m.slug === d.promoted?.into) ? (
                  <RefTag refKey={refKey({ kind: "mission", slug: d.promoted.into })} inline>
                    {d.promoted.into}
                  </RefTag>
                ) : (
                  <Text mono>{d.promoted.into}</Text>
                )
              ) : undefined
            }
          />
        )}
        {d.home && d.status === "promoted" && <HomeLine d={d} n={n} onDone={reload} />}
        {expiry && (expiry.soon || expiry.expired) && <ArtifactExpiryNote words={expiry.words} expired={expiry.expired} />}
        <ArtifactTools>
          <VersionStrip d={d} n={n} go={go} />
          <Segmented value={width} onChange={setWidth} disabled={source} aria-label="Width">
            {widths.map((w) => (
              <Segment key={w.key} value={w.key} title={w.title} keys={artifactViewKeys.width}>
                {w.label}
              </Segment>
            ))}
          </Segmented>
          <ArtifactActions
            runnable={runnable}
            source={source}
            stopped={stopped}
            manyFiles={!!v?.files && v.files > 1}
            onReload={again}
            onStop={() => setStopped((s) => !s)}
            onSource={setSource}
            tweaks={framed && <TweaksToggle told={told} shown={tray} onToggle={() => setTray((x) => !x)} keys={artifactViewKeys.tweaks} />}
            compare={
              comparing && {
                to: compareHref(comparing),
                title: comparing[0].id === d.id ? `Compare v${comparing[0].n} and v${comparing[1].n}` : `Compare with ${comparing[0].id}`,
              }
            }
            alone={runnable ? frameURL(d.id, n, opensAs) : undefined}
            keep={keepable ? { promoted: d.status === "promoted", onPromote: () => setDialog("promote"), onDiscard: () => setDialog("discard") } : undefined}
            onRestore={d.status === "discarded" && d.restorable ? () => restore(d).then((done) => done && reload()) : undefined}
          />
        </ArtifactTools>
      </ArtifactHead>
      <ArtifactBody>
        <ArtifactStage>
          {missing ? (
            <ArtifactEmpty icon="search" title={`${d.id} has no v${asked}`} action={<Link to={artifactHref(d.id)}>Latest: v{d.latest?.n}</Link>}>
              Versions: {d.all.map((x) => `v${x.n}`).join(", ")}.
            </ArtifactEmpty>
          ) : !runnable ? (
            <ArtifactEmpty icon="alert" title="No file to run">
              {d.gone ? `Folder gone from ${d.where.replace(/[^/]+\/$/, "")}. Only the record remains.` : `v${n}'s files are missing from ${d.where}.`}
            </ArtifactEmpty>
          ) : source ? (
            <Source id={d.id} n={n} />
          ) : stopped ? (
            <ArtifactEmpty icon="stop" title="Stopped" action={<Button onClick={() => setStopped(false)} keys={artifactViewKeys.stop}>Run</Button>}>
              Frame and scripts unloaded.
            </ArtifactEmpty>
          ) : (
            <ArtifactFrame src={frameSrc} title={`${d.title}, v${n}`} px={px} frameKey={`${d.id}.v${n}#${run}`} frameRef={tweaks.frame} />
          )}
        </ArtifactStage>
        {framed && tray && hasTweaks(told) && (
          <TweaksTray told={told} set={tweaks.set} onClose={() => setTray(false)} keys={artifactViewKeys.tweaks}>
            {changed(told).length > 0 && (
              <Link className={textClass("link")} to={compareHref([{ id: d.id, n }, { id: d.id, n }], [[], tweakParams(told)])} title="Side by side: defaults and current tweaks">
                <Icon name="split" size={12} /> Compare with defaults
              </Link>
            )}
          </TweaksTray>
        )}
      </ArtifactBody>
      {dialog === "promote" && <PromoteDialog a={d} version={v?.n} onClose={() => setDialog("")} onDone={() => reload()} />}
      {dialog === "discard" && <DiscardDialog a={d} onClose={() => setDialog("")} onDone={() => reload()} />}
    </ArtifactView>
  );
}
