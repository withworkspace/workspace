import { type ReactNode, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router";
import { Button as AriaButton, Menu, MenuItem, MenuTrigger, Popover } from "react-aria-components";
import {
  act,
  type ActVerb,
  type Artifact,
  type CapsuleDetail,
  type CrewDetail,
  type CrewPick,
  type CrewPicks,
  getTranscript,
  sayToCrew,
  type TranscriptEntry,
  getMission,
  type Headline,
  type Look,
  type MissionDetail,
  type Talk,
  useEvents,
  type Workspace,
} from "../api";
import { useKeys } from "../keys";
import { Markdown } from "../markdown/LazyMarkdown";
import { TalkList, talkHref } from "../talk/Talk";
import { Icon, type IconName } from "../ui/Icon";
import { CheckingNow, checkingTitle } from "./checking";
import { TLink } from "../ui/TLink";
import { Inline, plain } from "../ui/inline";
import { Callout, type CalloutTone, Copyable, Empty, ErrorCard, humanError, Keys, Skeleton, toast, useTitle } from "../ui/kit";
import { headlineStatus, missionStateStatus, Status } from "../ui/status";
import { Time } from "../ui/time";
import { RefContext } from "../ui/hover";
import { useRefs } from "../refs/known";
import { refAt, refKey } from "../refs/refs";
import { RefTag } from "../refs/RefTag";
import { PromptCard, pendingCall } from "./AgentView";
import { archiveMission } from "./archive";
import { briefGoal } from "./brief";
import { briefHref } from "./BriefView";
import { shownHeadlines } from "./headlines";
import { mediaKind } from "./media";
import { missionRefs } from "./refs";
import { ArtifactRow, CapsuleBadges, CrewRow, Decisions, Events, isArtifact, MissionData, Stepper, Timeline, viewEvent } from "./parts";
import { EndButton, EndDialog, EndedCapsules, ended, endingWords, HandedOn, ZChecks } from "./Ending";
import { PlanList } from "./PlanList";
import { waitHow } from "./planRows";
import { Verify } from "./Verify";
import { useRepoLabel, Work, workWords } from "./Work";
import { crewElsewhere, work } from "./workRows";
import { checkRows, rowKey, shownBundle, toApprove, waiting as bundleWaiting } from "./verifyRows";
import { type Reload, useLoaded } from "../load";
import { PagePin } from "../shell/focus";

// The mission page: its time along the top edge, then one bar (back, what
// it's doing, its name, six tabs) that stays as the page scrolls, and the
// tab under it. Summary leads with what needs you; Brief is the brief
// (editable); Work is its capsules and documents; Verify is its
// verification bundles, what crew handed over to show their work; Log is
// everything that's happened, and its crew; Talk is conversations about it.
// The details (state, stages, documents, capsules and crew) are beside
// Summary; on the other tabs, which want the room, s slides them in over
// the page. Summary, Brief, Log and Talk are read, in a column; Verify and
// Work take the page's width.

const tabs = ["summary", "brief", "work", "verify", "log", "talk"] as const;
type Tab = (typeof tabs)[number];
const tabNames: Record<Tab, string> = { summary: "Summary", brief: "Brief", work: "Work", verify: "Verify", log: "Log", talk: "Talk" };
// The tabs that are somewhere to work, not something to read.
const wideTabs: Tab[] = ["work", "verify"];

export const missionKeys: [string, string][] = [
  ["1–6", "Summary, Brief, Work, Verify, Log, Talk"],
  ["j / k", "Next / previous document or capsule, or a bundle's check"],
  ["⏎", "Open it: a capsule's review, a document's page"],
  ["v", "Review the capsule, or read the replies on a document"],
  ["o", "Open its link (a PR, a document on GitHub)"],
  ["f", "Browse the capsule's files (or the mission's only one)"],
  ["e", "Edit the brief and plan, on their own page"],
  ["s", "Show or hide the details: beside Summary, over the other tabs"],
  ["r", "Reload"],
  ["y / n", "On Verify: approve the check (or mark it looked at) / say it's not right"],
  ["mod+⏎", "On Verify: approve the bundle, or send it back with what you said"],
  ["esc", "Close the details or a bundle you opened, or back to the missions"],
];

const sideKey = "ws:mission-side";

// remembered is each mission's selection, kept while you open what's
// selected and come back.
const remembered = new Map<string, string>();

function loadSide(): boolean {
  try {
    return localStorage.getItem(sideKey) !== "0";
  } catch {
    return true;
  }
}

// Item is something on the page the cursor can be on.
interface Item {
  key: string;
  open: () => void;
  url?: string;
  capsule?: CapsuleDetail;
}

export function MissionPage({ workspace }: { workspace: Workspace }) {
  const { slug = "" } = useParams();
  const [params, setParams] = useSearchParams();
  const tab: Tab = tabs.includes(params.get("tab") as Tab) ? (params.get("tab") as Tab) : "summary";
  const navigate = useNavigate();
  const updated = workspace.missions.find((m) => m.slug === slug)?.updated;
  // What it's about, by name (a plugin's thing, a pull request): as the
  // workspace says of it.
  const about = workspace.missions.find((m) => m.slug === slug)?.about;
  // The details beside Summary (kept, as you left them), and over another
  // tab (until you leave it).
  const [side, setSide] = useState(loadSide);
  const [over, setOver] = useState(false);
  useEffect(() => setOver(false), [tab, slug]);
  const docked = tab === "summary" && side;

  // What's on disk first (fast), then again with the capsules' PRs (GitHub
  // is slow); again whenever the mission changes.
  const mission = useLoaded<MissionDetail>(
    (soon) =>
      getMission(slug, false).then((x) => {
        // (Stale by now, its answer's dropped: GitHub needn't be asked.)
        const wanted = soon((cur) => (cur?.github ? { ...x, capsules: withPRs(x.capsules, cur.capsules), github: true } : x));
        return wanted ? getMission(slug, true) : x;
      }),
    [slug],
    { keep: true },
  );
  const { value: d, error, reload: load } = mission;
  // The mission follows its own topic: told at once when its records
  // change, and of its files and capsules at the server's next look. A
  // stream opened again (the page shown again) catches up; the first is
  // what was just loaded.
  const opened = useRef(false);
  useEffect(() => {
    opened.current = false;
  }, [slug]);
  useEvents(`mission:${slug}`, {
    hello: () => {
      if (opened.current) load();
      opened.current = true;
    },
    changed: load,
  });
  useTitle(d?.title ?? "Mission");
  const refs = useMemo(() => (d ? missionRefs(d) : undefined), [d]);

  const openArtifact = useCallback(
    (a: Artifact) => {
      if (isArtifact(a)) navigate(`/artifacts/${a.ref}`);
      else if (a.readable) navigate(`/missions/${slug}/artifact?${new URLSearchParams({ ref: a.ref })}`);
      else if (a.url) window.open(a.url, "_blank", "noopener");
    },
    [navigate, slug],
  );
  const review = useCallback((c: CapsuleDetail) => c.exists && navigate(`/missions/${slug}/review/${c.repo}/${c.name}`), [navigate, slug]);
  // The bundle Verify has open: the one its address names (b), else the
  // latest standing.
  const picked = params.get("b") ?? undefined;
  const pickBundle = (id?: string) => setParams(id ? { tab: "verify", b: id } : { tab: "verify" }, { replace: true });
  const bundle = useMemo(() => (d ? shownBundle(d.bundles, picked) : undefined), [d, picked]);

  // What the Work tab lists, and the row it opens on.
  const w = useMemo(() => (d ? work(d) : undefined), [d]);
  const items = useMemo((): Item[] => {
    if (!d) return [];
    const caps = d.capsules.map((c): Item => ({ key: `c:${c.repo}/${c.name}`, open: () => review(c), url: c.pr?.url, capsule: c }));
    const arts = (list: Artifact[]) => list.map((a): Item => ({ key: `a:${a.ref}`, open: () => openArtifact(a), url: a.url }));
    switch (tab) {
      case "summary":
        // In the order the details (or Summary without them) show them.
        if (!side) return caps.filter((i) => w?.hand.some((x) => x.key === i.key));
        return [...arts(d.artifacts.slice(0, sideArtifacts)), ...caps];
      case "work": {
        // In the order its list has them: capsules as they need you, then outputs.
        const all = new Map([...caps, ...arts(d.artifacts)].map((i) => [i.key, i]));
        return (w?.keys ?? []).map((k) => all.get(k)).filter((i): i is Item => !!i);
      }
      case "verify":
        // The open bundle's checks, in the order it lists them.
        return bundle ? checkRows(bundle).map((r): Item => ({ key: rowKey(bundle, r), open: () => {} })) : [];
      case "log":
        return crewElsewhere(d)
          .filter((c) => c.handoff)
          .map((c): Item => ({ key: `h:${c.id}`, open: () => openArtifact({ ref: c.handoff!, readable: true } as Artifact) }));
    }
    return [];
  }, [d, w, tab, side, review, openArtifact, bundle]);
  const [sel, setSelState] = useState<string | undefined>(() => remembered.get(slug));
  const setSel = (key: string) => {
    remembered.set(slug, key);
    setSelState(key);
  };
  // Nothing's selected until j or k: the first j takes the first item. But
  // Work always has a row open: the one it opens on, until one's picked.
  const chosen = items.findIndex((i) => i.key === sel);
  const at = chosen < 0 && tab === "work" ? items.findIndex((i) => i.key === w?.opensOn) : chosen;
  const item = at >= 0 ? items[at] : undefined;
  useEffect(() => {
    if (!item) return;
    (document.getElementById(`item-${item.key}`) ?? document.getElementById(`side-${item.key}`))?.scrollIntoView({ block: "nearest" });
  }, [item?.key]);

  const move = (to: number) => items.length && setSel(items[Math.min(Math.max(to, 0), items.length - 1)].key);
  const setTab = (t: Tab) => setParams(t === "summary" ? {} : { tab: t }, { replace: true });
  const toggleSide = () =>
    setSide((v) => {
      try {
        localStorage.setItem(sideKey, v ? "0" : "1");
      } catch {
        // kept for this page only
      }
      return !v;
    });
  const back = () => navigate("/");
  useKeys({
    "1": () => setTab("summary"),
    "2": () => setTab("brief"),
    "3": () => setTab("work"),
    "4": () => setTab("verify"),
    "5": () => setTab("log"),
    "6": () => setTab("talk"),
    j: () => move(at + 1),
    ArrowDown: () => move(at + 1),
    k: () => move(at < 0 ? 0 : at - 1),
    ArrowUp: () => move(at < 0 ? 0 : at - 1),
    Enter: () => item?.open(),
    o: () => (item?.url ? window.open(item.url, "_blank", "noopener") : item?.open()),
    v: () => {
      // Replies waiting for you on a document, when that's where it stands.
      const h = d?.headlines[0];
      if (!item?.capsule && h?.action.verb === "review-doc" && h.action.target) return navigate(docLink(slug, h.action.target));
      // A Review step's first look: a bundle to approve, a document to
      // read, or a capsule (below). So is the bundle that shows a step's
      // checks, when they're yours to say.
      // One you've approved already isn't what it waits on.
      const looks = h?.action.verb === "review-step" || h?.action.verb === "answer" ? h.action.look : undefined;
      const look = looks?.find((l) => !l.approved) ?? looks?.[0];
      if (!item?.capsule && look?.bundle) return pickBundle(look.bundle);
      if (!item?.capsule && look?.doc) return navigate(docLink(slug, look.doc));
      const only = d?.capsules.filter((c) => c.exists);
      // The capsule that's back for re-review, or ready to land.
      const named = d?.reReview?.length === 1 ? d.reReview[0].capsule : h?.action.verb === "land" ? h.action.target : look?.capsule;
      const waiting = named ? d?.capsules.find((c) => `${c.repo}/${c.name}` === named) : undefined;
      const c = item?.capsule ?? waiting ?? (only?.length === 1 ? only[0] : undefined);
      if (c) review(c);
    },
    e: () => navigate(briefHref(slug)),
    f: () => {
      const only = d?.capsules.filter((c) => c.exists);
      const c = item?.capsule ?? (only?.length === 1 ? only[0] : undefined);
      if (c) navigate(`/missions/${slug}/files/${c.repo}/${c.name}`);
      else if (only?.length) setTab("work");
    },
    s: () => (tab === "summary" ? toggleSide() : setOver((v) => !v)),
    r: load,
    Escape: () => {
      if (over && tab !== "summary") return setOver(false);
      // A bundle opened when none is yours to approve closes, to Verify saying so.
      if (tab === "verify" && picked && d && !toApprove(d.bundles).length) return pickBundle();
      back();
    },
    h: back,
  });

  // The bar's ends, the same while the mission loads as once it has: back
  // to the missions, and the tabs (with what's waiting in each, once known).
  const backLink = (
    <TLink to="/" className="mission-back" title="Missions (esc)" aria-label="Back to the missions">
      <Icon name="arrow-left" size={15} />
    </TLink>
  );
  const tabRow = (count?: (t: Tab) => ReactNode) => (
    <div className="tabs" role="tablist" aria-label="Mission">
      {tabs.map((t) => (
        <button
          key={t}
          role="tab"
          id={`tab-${t}`}
          aria-selected={t === tab}
          aria-controls="mission-panel"
          className={t === tab ? "on" : ""}
          onClick={() => setTab(t)}
          title={`${tabNames[t]} (${tabs.indexOf(t) + 1})`}
        >
          {tabNames[t]}
          {count?.(t)}
        </button>
      ))}
    </div>
  );

  if (error && !d) {
    return (
      <div className="page narrow">
        <Crumbs />
        <ErrorCard
          title={/not found|no mission/i.test(error) ? "There's no mission here" : "Couldn't load this mission"}
          error={/not found|no mission/i.test(error) ? `No mission is called ${slug}: it may have been renamed or deleted.` : error}
          retry={load}
        />
        <Link to="/" className="button-link">
          <Icon name="arrow-left" /> All missions
        </Link>
      </div>
    );
  }
  if (!d) {
    // The page's own frame while it loads, so nothing moves when it has:
    // the bar where it will be, with what the list already says of the
    // mission (where it stands, its name) and the tabs.
    const listed = workspace.missions.find((m) => m.slug === slug);
    return (
      <div className="mission-page">
        <div className="timeline" aria-hidden>
          <div className="timeline-track" />
        </div>
        <div className="mission-bar">
          <div className="mission-bar-in">
            <header className="mission-head">
              {backLink}
              {listed && <Status s={headlineStatus(listed.headline)} iconOnly size={16} className="mission-status" />}
              {listed ? (
                <h1 className="mission-title">
                  <span className="mission-title-text" title={listed.title}>
                    {listed.title}
                  </span>
                </h1>
              ) : (
                <h1 className="skeleton-title">&nbsp;</h1>
              )}
            </header>
            {tabRow()}
          </div>
        </div>
        <div className="page mission">
          <div className="mission-main">
            <Skeleton rows={6} />
          </div>
        </div>
      </div>
    );
  }
  const selected = (key: string) => (item?.key === key ? " selected" : "");
  // Your turn in them, or something they proposed to confirm.
  const waitingTalks = d.talks.filter((x) => x.waiting || !!x.proposed).length;

  // Bundles waiting for you, and those nothing has taken the place of:
  // what Verify has to show.
  const waitingBundles = d.bundles.filter(bundleWaiting).length;
  const standing = d.bundles.filter((b) => !b.replacedBy).length;

  const parts = { d, selected, openArtifact, review, go: setTab, reload: load };
  return (
    <MissionData.Provider value={d}>
    <RefContext.Provider value={refs}>
    <div className={`mission-page${tab === "verify" || tab === "work" ? " fills" : ""}`}>
      <Timeline d={d} />
      <div className="mission-bar">
        <div className="mission-bar-in">
          <header className="mission-head">
            {backLink}
            {d.headlines[0] && <Status s={headlineStatus(d.headlines[0])} iconOnly size={16} className="mission-status" />}
            <Title d={d} reload={load} />
            <PagePin r={{ kind: "mission", slug: d.slug }} />
          </header>
          {tabRow((t) => (
            <>
              {t === "verify" && waitingBundles > 0 && (
                <span className="count tone-attention" title="Verification bundles waiting for you to approve">
                  {waitingBundles}
                </span>
              )}
              {t === "verify" && !waitingBundles && standing > 0 && (
                <span className="count" title="Verification bundles, handed over to show the work">
                  {standing}
                </span>
              )}
              {t === "talk" && waitingTalks > 0 && (
                <span className="count tone-attention" title="Your turn, or something to confirm">
                  {waitingTalks}
                </span>
              )}
            </>
          ))}
        </div>
        {tab !== "summary" && <DetailSide {...parts} updated={updated} about={about} over open={over} />}
      </div>
      <div className={`page mission${docked ? " with-side" : ""}${wideTabs.includes(tab) ? " wide" : ""}`}>
        <div className="mission-main">
          {error && <ErrorCard title="Couldn't refresh" error={error} retry={load} />}
          <div role="tabpanel" id="mission-panel" aria-labelledby={`tab-${tab}`} className="tab-panel" key={tab}>
            {tab === "summary" && <Summary {...parts} full={!side} />}
            {tab === "brief" && <Brief d={d} reload={load} />}
            {tab === "work" && w && (
              <Work d={d} w={w} shown={item?.key} select={setSel} review={review} openArtifact={openArtifact} reload={load} />
            )}
            {tab === "verify" && <Verify d={d} bundle={bundle} pick={pickBundle} sel={item?.key} select={setSel} reload={load} />}
            {tab === "log" && <Log {...parts} />}
            {tab === "talk" && <MissionTalk slug={d.slug} talks={d.talks} planner={d.planner} />}
          </div>
        </div>
        {docked && <DetailSide {...parts} updated={updated} about={about} />}
      </div>
    </div>
    </RefContext.Provider>
    </MissionData.Provider>
  );
}

function Crumbs() {
  return (
    <nav className="crumbs" aria-label="Breadcrumb">
      <TLink to="/">
        <Icon name="arrow-left" size={13} /> Missions
      </TLink>
    </nav>
  );
}

// Title is the mission's name, renamed in place (a pencil on hover).
function Title({ d, reload }: { d: MissionDetail; reload: Reload }) {
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState(d.title);
  const save = () => {
    const t = value.trim();
    setEditing(false);
    if (!t || t === d.title) return;
    act(d.slug, "title", { text: t }).then(
      (r) => {
        toast(r.message, { tone: "done" });
        reload();
      },
      (e: Error) => toast(humanError(e.message), { tone: "danger" }),
    );
  };
  if (editing) {
    return (
      <input
        className="title-input"
        autoFocus
        value={value}
        maxLength={50}
        aria-label="Mission title"
        onChange={(e) => setValue(e.target.value)}
        onBlur={save}
        onKeyDown={(e) => {
          if (e.key === "Enter") save();
          if (e.key === "Escape") {
            e.preventDefault();
            setValue(d.title);
            setEditing(false);
          }
        }}
      />
    );
  }
  return (
    <h1 className="mission-title">
      <span className="mission-title-text" title={d.title}>
        {d.title}
      </span>
      <button
        className="ghost icon-button small rename"
        onClick={() => {
          setValue(d.title);
          setEditing(true);
        }}
        title="Rename"
        aria-label="Rename the mission"
      >
        <Icon name="pencil" size={12} />
      </button>
    </h1>
  );
}

// ParkButton puts a mission aside, out of what needs you and out of the
// orchestrator's way, until it's unparked; and unparks a parked one.
function ParkButton({ d, reload }: { d: MissionDetail; reload: Reload }) {
  const [busy, setBusy] = useState(false);
  if (d.state === "archived" || d.state === "briefing") return null;
  const parked = d.state === "parked";
  const run = () => {
    setBusy(true);
    act(d.slug, parked ? "unpark" : "park")
      .then(
        (r) => {
          toast(r.message, { tone: "done" });
          return reload();
        },
        (e: Error) => toast(humanError(e.message), { tone: "danger" }),
      )
      .finally(() => setBusy(false));
  };
  return (
    <button className="ghost small" disabled={busy} onClick={run} title={parked ? "Take it off parked" : "Put it aside until you unpark it"}>
      <Icon name={parked ? "play" : "pause"} size={12} /> {parked ? "Unpark" : "Park"}
    </button>
  );
}

// DetailSide is the mission's details: what it is, where its stages are,
// its documents and capsules, who's working. Beside Summary; over another
// tab (over) it's a card that slides in from the right when open, there
// all along so it can slide out again.
function DetailSide({ d, selected, openArtifact, review, updated, about, over, open, go, reload }: Parts & { updated?: string; about?: string[]; over?: boolean; open?: boolean }) {
  const live = d.crew.filter((c) => !c.ended);
  const accepted = d.stages.filter((s) => s.state === "accepted").length;
  return (
    <aside className={`detail-side${over ? " over" : ""}${over && open ? " open" : ""}`} aria-label="Details" inert={over && !open}>
      <div className="side-head">
        <span>Details</span>
        <Keys k="s" />
      </div>
      <dl className="props">
        <dt className="sr-only">State</dt>
        <dd className="state-row">
          <Status s={missionStateStatus(d.state)} />
          <ParkButton d={d} reload={reload} />
          <EndButton d={d} reload={reload} />
        </dd>
        {d.at && (
          <>
            <dt>At</dt>
            <dd className="at-row">{d.at}</dd>
          </>
        )}
        <dt>Created</dt>
        <dd>
          <Time iso={d.created} />
        </dd>
        {updated && (
          <>
            <dt>Updated</dt>
            <dd>
              <Time iso={updated} as="relative" />
            </dd>
          </>
        )}
        <dt>Slug</dt>
        <dd>
          <Copyable text={d.slug} className="mono" />
        </dd>
        {about && about.length > 0 && (
          <>
            <dt>About</dt>
            <dd className="about-row">
              {about.map((name) => (
                <About key={name} name={name} />
              ))}
            </dd>
          </>
        )}
        {d.from && (
          <>
            <dt>Raised from</dt>
            <dd className="mono">{d.from}</dd>
          </>
        )}
        {d.conversation && (
          <>
            <dt>Came from</dt>
            <dd>
              <Link to={cameFrom(d.conversation)} className="link came-from">
                <Icon name="message" size={12} />
                {d.conversationTitle || "a conversation"}
              </Link>
            </dd>
          </>
        )}
      </dl>
      {d.stages.length > 0 && (
        <SideSection title={d.plan ? "Plan" : "Stages"} count={`${accepted} of ${d.stages.length} done`}>
          <PlanList stages={d.stages} />
        </SideSection>
      )}
      {d.artifacts.length > 0 && (
        <SideSection title="Outputs" count={d.artifacts.length}>
          {d.artifacts.slice(0, sideArtifacts).map((a) => {
            const key = `a:${a.ref}`;
            return <ArtifactRow key={key} id={`side-${key}`} a={a} compact selected={!!selected(key)} open={openArtifact} />;
          })}
          {d.artifacts.length > sideArtifacts && (
            <button className="link more-link" onClick={() => go("work")}>
              {d.artifacts.length - sideArtifacts} more in Work
            </button>
          )}
        </SideSection>
      )}
      <SideSection title="Capsules" count={d.capsules.length || undefined}>
        {d.capsules.map((c) => {
          const key = `c:${c.repo}/${c.name}`;
          return (
            <button key={key} id={`side-${key}`} className={`side-capsule${selected(key)}`} onClick={() => review(c)} title={c.exists ? "Review it (v)" : "Burned: its folder's gone"} disabled={!c.exists}>
              <span className="side-capsule-name">
                {c.dirty && <span className="dirty-dot" title="Uncommitted changes" />}
                <span className="mono">
                  {c.repo}/{c.name}
                </span>
              </span>
              {(!!c.ahead || !!c.behind || c.pr || !c.exists) && <CapsuleBadges c={{ ...c, dirty: false }} />}
            </button>
          );
        })}
        {!d.capsules.length && !d.endedCapsules?.length && <div className="side-none">None yet: crew lift one when they start.</div>}
        <EndedCapsules list={d.endedCapsules} />
      </SideSection>
      {d.runs.length > 0 && (
        <SideSection title="Orchestrator" count={d.runs.length}>
          {d.runs.slice(0, 4).map((r) => (
            <Link key={r.id} to={`/missions/${d.slug}/orchestrator/${r.id}`} className="side-run">
              <Status
                s={
                  r.error
                    ? { icon: "x-circle", tone: "danger", words: "Failed" }
                    : r.acted
                      ? { icon: "check-circle", tone: "done", words: "Acted" }
                      : { icon: "circle-dashed", tone: "quiet", words: "Did nothing" }
                }
              />
              <span className="side-run-for">on {r.for.replace(/ \(([^)]+)\) \(\1\)$/, " ($1)")}</span>
              <Time iso={r.started} as="age" className="age" />
            </Link>
          ))}
        </SideSection>
      )}
      <SideSection title="Crew" count={live.length ? `${live.length} on it` : undefined}>
        {live.map((c) => (
          <CrewRow key={c.id} c={c} slug={d.slug} />
        ))}
        {d.crew.length > live.length && (
          <button className="link more-link" onClick={() => go("work")}>
            {live.length ? `${d.crew.length - live.length} earlier, in Work` : `${d.crew.length} so far, none on it now: in Work`}
          </button>
        )}
        {!d.crew.length && <div className="side-none">Nobody yet.</div>}
      </SideSection>
    </aside>
  );
}

function SideSection({ title, count, children }: { title: string; count?: string | number; children: React.ReactNode }) {
  return (
    <section className="side-section">
      <h3>
        {title}
        {count !== undefined && <span className="side-count">{count}</span>}
      </h3>
      {children}
    </section>
  );
}

const sideArtifacts = 6;
const summaryRecent = 5;
const summaryRecentWithSide = 7;
const summaryDecisions = 3;

// withPRs keeps the PRs already known while what's on disk refreshes.
function withPRs(next: CapsuleDetail[], known: CapsuleDetail[]): CapsuleDetail[] {
  return next.map((c) => ({ ...c, pr: c.pr ?? known.find((k) => k.repo === c.repo && k.name === c.name)?.pr }));
}

interface Parts {
  d: MissionDetail;
  selected: (key: string) => string;
  openArtifact: (a: Artifact) => void;
  review: (c: CapsuleDetail) => void;
  go: (t: Tab) => void;
  reload: Reload;
}

// About is one thing a mission's about, by its name: drawn as what it
// names (a plugin's thing, a pull request, anything ws names), with its
// card, and as its words where ws doesn't read it as a name.
function About({ name }: { name: string }) {
  const known = useRefs()?.known;
  const ref = known ? refAt(name, known) : undefined;
  return ref ? <RefTag refKey={refKey(ref)}>{name}</RefTag> : <span className="mono">{name}</span>;
}

// A harness's warning the orchestrator passed on as what it said ("this
// sign-in ends between …"): about the whole workspace, not the mission,
// so it's shown once, dismissably, and kept out of Recently.
const isNotice = (text: string) => /^orchestrator: (warning|notice): /i.test(text);

function Notices({ d }: { d: MissionDetail }) {
  const texts = [...new Set(d.events.filter((e) => isNotice(e.text)).map((e) => e.text.replace(/^orchestrator: /, "")))].slice(-1);
  const [dismissed, setDismissed] = useState(() => {
    try {
      return new Set<string>(JSON.parse(localStorage.getItem("ws-dismissed") ?? "[]"));
    } catch {
      return new Set<string>();
    }
  });
  const shown = texts.filter((t) => !dismissed.has(t.slice(0, 80)));
  if (!shown.length) return null;
  const dismiss = (t: string) => {
    const next = new Set(dismissed).add(t.slice(0, 80));
    setDismissed(next);
    try {
      localStorage.setItem("ws-dismissed", JSON.stringify([...next].slice(-50)));
    } catch {
      // dismissed for this page only
    }
  };
  return (
    <>
      {shown.map((t) => {
        const signIn = /sign-in ends/i.test(t);
        return (
          <Callout
            key={t}
            tone="quiet"
            icon="alert"
            title={signIn ? "The agents' sign-in ends soon" : "The orchestrator's agent warned"}
            actions={
              <button className="small" onClick={() => dismiss(t)}>
                Dismiss
              </button>
            }
          >
            <Inline text={t.replace(/^(warning|notice): (\w)/i, (_, _w: string, c: string) => c.toUpperCase())} />
          </Callout>
        );
      })}
    </>
  );
}

// Summary is what needs you, then the rest in a line or two. It leads
// with where the mission stands: the step it's on, what's happening and
// what moves it on, and the last word from its crew, as one block. With
// the details showing, stages, crew, capsules and documents are there, and
// the rest keeps to the goal and what's happened.
function Summary({ d, selected, review, full, go, reload }: Parts & { full: boolean }) {
  const goal = briefGoal(d.brief);
  const w = useMemo(() => work(d), [d]);
  const label = useRepoLabel();
  const live = d.crew.filter((c) => !c.ended);
  const now = d.stages.find((s) => s.state === "current");
  const events = d.events.filter((e) => !isNotice(e.text));
  const recent = full ? summaryRecent : summaryRecentWithSide;
  // The step it's on, and what crew last said: in the lead when there's a
  // headline to lead with, else rows of their own.
  const led = d.headlines.length > 0;
  const step = now && (
    <>
      <span title={now.text}>
        {d.plan ? "Step" : "Stage"} {now.n} of {d.stages.length}: <strong>{now.title}</strong>
      </span>
      {now.checkIn && <span className="dim"> · check in after</span>}
      {now.who === "you" && !waitHow(now) && <span className="dim"> · yours: only you pass it</span>}
      {waitHow(now) && (
        <span className="lead-wait">
          {" "}
          · <span className={`chip ${waitHow(now)!.auto ? "tone-progress" : "tone-attention"}`}>{waitHow(now)!.word}</span> <span className="dim">{waitHow(now)!.says}</span>
        </span>
      )}
    </>
  );
  const said = live.filter((c) => c.lastReply && c.activity !== "working");
  return (
    <>
      <Headlines
        d={d}
        reload={reload}
        step={step}
        latest={
          said.length > 0 && (
            <dl className="lead-latest">
              {said.map((c) => (
                <div key={c.id} className="lead-said">
                  <dt>From {c.id}</dt>
                  <dd>
                    <Said text={c.lastReply!} />
                  </dd>
                </div>
              ))}
            </dl>
          )
        }
      />
      <Notices d={d} />
      <dl className="summary">
        {goal && (
          <Row label="Goal">
            <Inline text={goal} />{" "}
            <button className="link more-link" onClick={() => go("brief")}>
              Brief
            </button>
          </Row>
        )}
        {!led && !full && step && <Row label="Now">{step}</Row>}
        {!led &&
          said.map((c) => (
            <Row key={c.id} label={`From ${c.id}`}>
              <Said text={c.lastReply!} />
            </Row>
          ))}
        {full && d.stages.length > 0 && (
          <Row label={d.plan ? "Plan" : "Stages"}>
            <Stepper stages={d.stages} />
          </Row>
        )}
        {full && (
          <Row label="Work">
            {w.hand.map((x) => (
              <CapsuleCard key={x.key} c={x.c} selected={selected} review={review} />
            ))}
            <span className="dim summary-work">
              {workWords(w, label, d.slug) || "Nothing yet: crew lift a capsule when they start."}{" "}
              {(w.capsules.length > 0 || w.outputs.length > 0) && (
                <button className="link more-link" onClick={() => go("work")}>
                  All of it in Work
                </button>
              )}
            </span>
          </Row>
        )}
        {full && live.length > 0 && (
          <Row label="Crew">
            {live.map((c) => (
              <CrewRow key={c.id} c={c} slug={d.slug} />
            ))}
          </Row>
        )}
        {d.decisions.some((x) => x.state === "current") && (
          <Row label="Decided">
            <Decisions list={d.decisions} limit={summaryDecisions} />
          </Row>
        )}
        {events.length > 0 && (
          <Row label="Recently">
            <Events events={events.slice(-recent)} lines={2} />
            {events.length > recent && (
              <button className="link more-link" onClick={() => go("log")}>
                The whole log ({events.length})
              </button>
            )}
          </Row>
        )}
        {d.from && (
          <Row label="Raised from">
            <span className="dim">{d.from}</span>
          </Row>
        )}
      </dl>
    </>
  );
}

// Said is what crew last said, rendered, a few lines of it until you ask
// for the rest.
function Said({ text }: { text: string }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const [long, setLong] = useState(false);
  useEffect(() => {
    const el = ref.current;
    if (el) setLong(el.scrollHeight > el.clientHeight + 4);
  }, [text]);
  return (
    <div className="said-block">
      <div ref={ref} className={`said-body${open ? " open" : ""}`}>
        <Markdown text={text} />
      </div>
      {(long || open) && (
        <button className="link more-link" onClick={() => setOpen((o) => !o)}>
          {open ? "Show less" : "Show all"}
        </button>
      )}
    </div>
  );
}

// Headlines lead the Summary: where the mission stands (whose court it's
// in, and what moves it on), then anything else raised: all that's yours
// or outside's, and one banner for the agents at work, however many are.
// ws mission show says the same, from the same place (mission.Resolve).
function Headlines({ d, reload, step, latest }: { d: MissionDetail; reload: Reload; step?: React.ReactNode; latest?: React.ReactNode }) {
  return (
    <>
      {shownHeadlines(d.headlines).map((h, i) => (
        <HeadlineCard key={`${h.kind}-${h.by ?? ""}-${i}`} d={d} h={h} primary={i === 0} reload={reload} step={i === 0 ? step : undefined} latest={i === 0 ? latest : undefined} />
      ))}
    </>
  );
}

// Lead is the first headline, as the Summary's subject rather than a
// notice on it: the step the mission's on over what's happening, at a
// title's size; what moves it on beside that when it's one thing to do, or
// under it when it's choices and an answer; and crew's last word, so "what
// now, and what's mine" is one block. Its tone is a wash, not an edge.
function Lead({
  tone,
  icon,
  live,
  title,
  step,
  actions,
  latest,
  children,
}: {
  tone: CalloutTone;
  icon: IconName;
  live?: boolean;
  title: React.ReactNode;
  step?: React.ReactNode;
  actions?: React.ReactNode;
  latest?: React.ReactNode;
  children?: React.ReactNode;
}) {
  return (
    <section className={`lead tone-${tone}`} role={tone === "danger" ? "alert" : undefined} aria-label="Where it stands">
      <div className="lead-main">
        <div className="lead-body">
          {step && <div className="lead-step">{step}</div>}
          <div className="lead-line">
            <span className={`status tone-${tone}${live ? " live" : ""} lead-icon`}>
              <Icon name={icon} size={20} />
            </span>
            <h2 className="lead-title">{title}</h2>
          </div>
          <div className="lead-text">{children}</div>
        </div>
        {actions && <div className="lead-actions">{actions}</div>}
      </div>
      {latest}
    </section>
  );
}

const courtTone = (h: Headline, tone: string): CalloutTone => {
  if (tone === "danger" || tone === "done") return tone;
  if (h.court === "you") return h.group === "needs-you" && h.kind !== "stalled" ? "attention" : "quiet";
  if (h.court === "agent") return "progress";
  if (h.court === "outside") return "info";
  return "quiet";
};

// splitHeadline makes a headline's sentence a title and what follows:
// "Looks finished: everything in the brief is met…".
function splitHeadline(h: Headline, words: string): [string, string | undefined] {
  const text = h.text.trim();
  if (h.kind === "resume") {
    const m = /^Resume (\S+) with: ([\s\S]+)$/.exec(text);
    if (m) return [`Suggested: resume ${m[1]}`, m[2]];
  }
  const m = /^([^:\n]{3,90}?): ([\s\S]+)$/.exec(text);
  if (m) return [m[1], /^[a-z]+\b/.test(m[2]) && !/^[a-z]+\d/.test(m[2]) ? m[2][0].toUpperCase() + m[2].slice(1) : m[2]];
  if (text.length <= 100) return [text, undefined];
  return [words, text];
}

function HeadlineCard({ d, h, primary, reload, step, latest }: { d: MissionDetail; h: Headline; primary: boolean; reload: Reload; step?: React.ReactNode; latest?: React.ReactNode }) {
  const st = headlineStatus(h);
  const [title, detail] = h.checking ? [checkingTitle(h.checking), undefined] : splitHeadline(h, st.words);
  const long = !!detail && detail.length > 220;
  const options = (h.kind === "flag" && !primary) || h.kind === "crew-asks" ? h.action.options : undefined;
  const actions = primary || h.action.verb === "resume" ? <HeadlineAct d={d} h={h} reload={reload} /> : undefined;
  const body = (
    <>
      {h.checking && <CheckingNow c={h.checking} />}
      {detail &&
        (h.kind === "resume" || (long && !primary) ? (
          <details className="fold">
            <summary>{h.kind === "resume" ? `What ${h.action.target ?? "crew"} will be told` : "Read it all"}</summary>
            <Inline text={detail} />
          </details>
        ) : long ? (
          <More text={detail} lines={3} />
        ) : (
          <Inline text={detail} />
        ))}
      {!!options?.length && (
        <ol className="options">
          {options.map((o, j) => (
            <li key={j}>{o}</li>
          ))}
        </ol>
      )}
      {primary && (h.kind === "to-brief" || h.kind === "briefing") && d.request && <div className="dim request">Requested: {d.request}</div>}
      {h.kind === "wrap-up" && d.conclude?.ending && (
        <>
          <ZChecks c={d.conclude} />
          <HandedOn to={d.conclude.to} />
        </>
      )}
      {h.kind === "ended" && d.ending && (
        <>
          <ZChecks c={d.ending} />
          <HandedOn to={d.ending.to} />
        </>
      )}
    </>
  );
  // The first leads the page; the rest are notices under it.
  return primary ? (
    <Lead tone={courtTone(h, st.tone)} icon={st.icon} live={st.live} title={title} step={step} actions={actions} latest={latest}>
      {body}
    </Lead>
  ) : (
    <Callout tone="quiet" icon={st.icon} title={title} className="headline also" actions={actions}>
      {body}
    </Callout>
  );
}

// More is text clamped to a few lines, with the rest a click away.
function More({ text, lines }: { text: string; lines: number }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Inline text={text} lines={open ? undefined : lines} />{" "}
      <button className="link more-link" onClick={() => setOpen((o) => !o)}>
        {open ? "Less" : "More"}
      </button>
    </>
  );
}

// pickName is a harness and model as [crew] allowed writes it.
const pickName = (p: CrewPick) => (p.model ? `${p.harness}:${p.model}` : p.harness);

// LaunchCrew is the Launch crew button: it starts crew on crew's default
// agent and model. Where [crew] allows others, it's a split button, whose
// menu starts them on one of those instead (or the default).
function LaunchCrew({
  picks,
  primary,
  again,
  busy,
  launch,
}: {
  picks?: CrewPicks;
  primary: boolean;
  again: boolean;
  busy: boolean;
  launch: (p?: CrewPick) => void;
}) {
  const others = picks?.allowed ?? [];
  const main = (
    <button className={primary ? "primary" : ""} disabled={busy} onClick={() => launch()} title={picks && others.length ? `On ${pickName(picks.default)}` : undefined}>
      <Icon name="play" /> Launch {again ? "new " : ""}crew
    </button>
  );
  if (!picks || !others.length) return main;
  const all = [picks.default, ...others];
  // data-field: keys typed in the menu are its own, not the page's (keys.ts).
  return (
    <div className="split-button">
      {main}
      <MenuTrigger>
        <AriaButton className={`split-toggle${primary ? " primary" : ""}`} isDisabled={busy} aria-label="Launch crew on another agent or model" data-field>
          <Icon name="chevron-down" size={13} />
        </AriaButton>
        <Popover className="select-popover" placement="bottom end" offset={4} data-field>
          <Menu className="select-list" aria-label="Launch crew on" onAction={(k) => launch(k === "0" ? undefined : all[Number(k)])}>
            {all.map((p, i) => (
              <MenuItem key={pickName(p)} id={String(i)} textValue={pickName(p)} className="select-option split-pick">
                <span className="split-pick-name mono">
                  {p.harness}
                  {p.model ? ` · ${p.model}` : ""}
                </span>
                {!p.model && <span className="dim">its own model</span>}
                {i === 0 && <span className="chip">default</span>}
              </MenuItem>
            ))}
          </Menu>
        </Popover>
      </MenuTrigger>
    </div>
  );
}

// WaitActs is what moves a Wait step on from the page. Marking it done
// passes it, with a note (what you did, or what happened) kept for the
// crew after. A step ws watches can be looked at now; marked done, one of
// yours is looked at first, and passed as it is only when you say so.
function WaitActs({ d, yours, passIt, busy, run }: { d: MissionDetail; yours?: boolean; passIt?: boolean; busy: boolean; run: (verb: ActVerb, text?: string) => void }) {
  const [note, setNote] = useState("");
  const now = d.stages.find((s) => s.state === "current");
  const watched = now?.passes === "watched";
  const said = note.trim();
  const done = () => (yours ? run("answer", said ? `Done: ${said}` : "Done") : run("accept", said));
  // Watched and not yours, ws passes it: looking now is the thing to do,
  // and marking it done passes it whatever its check says.
  const lookFirst = watched && !yours;
  return (
    <div className="answer wait-acts">
      <form
        className="own-answer"
        onSubmit={(e) => {
          e.preventDefault();
          done();
        }}
      >
        <input
          value={note}
          onChange={(e) => setNote(e.target.value)}
          placeholder={yours ? "What you did, if the crew after should know" : "What happened, if the crew after should know"}
          disabled={busy}
          aria-label="A note for the crew after (optional)"
        />
        <button type="submit" className={lookFirst ? undefined : "primary"} disabled={busy} title={lookFirst ? "Pass the step as it is, whatever its check says" : watched ? "ws looks, and passes the step if it's happened" : "Pass the step"}>
          <Icon name="check" /> {lookFirst ? "Mark done anyway" : "Mark done"}
        </button>
      </form>
      {(watched || passIt) && (
        <div className="button-row">
          {watched && (
            <button className={lookFirst ? "primary" : undefined} disabled={busy} onClick={() => run("look")} title="Run its check now, rather than when ws next would">
              <Icon name="rotate" /> Look now
            </button>
          )}
          {passIt && (
            <button className="ghost" disabled={busy} onClick={() => run("answer", "Pass it")} title="Pass the step as it is, whatever its check says">
              Pass it as it is
            </button>
          )}
        </div>
      )}
    </div>
  );
}

// HeadlineAct is what moves a headline on, as far as this page can do it;
// talking to crew is on their crew page.
function HeadlineAct({ d, h, reload }: { d: MissionDetail; h: Headline; reload: Reload }) {
  const a = h.action;
  const navigate = useNavigate();
  const [busy, setBusy] = useState(false);
  const [own, setOwn] = useState("");
  const run = (verb: ActVerb, text?: string, crew?: string, pick?: CrewPick) => {
    setBusy(true);
    // An answer answers the task its headline is: a mission can be waiting on
    // you for several.
    const task = verb === "answer" && a.target ? Number(a.target) || undefined : undefined;
    // Archiving asks first when it would burn work that hasn't merged.
    (verb === "archive" ? archiveMission(d.slug) : act(d.slug, verb, { text, crew, task, harness: pick?.harness, model: pick?.model }))
      .then((r) => {
        if (!r) return;
        if (r.path) return navigate(r.path);
        toast(r.message, {
          tone: "done",
          action:
            verb === "archive"
              ? {
                  label: "Undo",
                  run: () =>
                    act(d.slug, "unarchive").then(
                      (x) => {
                        toast(x.message);
                        reload();
                      },
                      (e: Error) => toast(humanError(e.message), { tone: "danger" }),
                    ),
                }
              : undefined,
        });
        return reload();
      })
      .catch((e: Error) => toast(humanError(e.message), { tone: "danger" }))
      .finally(() => setBusy(false));
  };
  const label = (s: string) => s[0].toUpperCase() + s.slice(1);
  switch (a.verb) {
    case "end":
      return <EndAct d={d} reload={reload} />;
    case "answer": {
      // A question can be put to crew, or dismissed; a step's task is
      // answered, or passed.
      const t = d.tasks?.find((x) => String(x.n) === a.target);
      // A Wait on you: marked done, with what you did if it's worth saying.
      if (t?.verb === "do") return <WaitActs d={d} yours passIt={a.options?.includes("Pass it")} busy={busy} run={run} />;
      const question = !t || (t.verb === "answer" && !t.step);
      // A step's checks, yours to say: the bundles that show them are
      // what to look at, on the Verify tab, where approving one passes
      // the step and saying what's not right sends it back.
      // One you've approved already is there to look at again: with
      // nothing left to approve, what ws suggests first (passing it)
      // is what moves it on.
      const bundles = (a.look ?? []).filter((l) => l.bundle);
      const toVerify = bundles.filter((l) => !l.approved);
      return (
        <div className="answer">
          <div className="button-row">
            {toVerify.map((l, i) => (
              <Link key={l.bundle} to={bundleLink(d.slug, l.bundle!)} className={i === 0 ? "button-link primary" : "button-link"} title="Look at what it shows: approving it passes the step">
                <Icon name="list-checks" /> Verify {l.name} {i === 0 && <Keys k="v" />}
              </Link>
            ))}
            {a.options?.map((o, i) => (
              <button key={i} className={bundles.length > 0 && toVerify.length === 0 && i === 0 ? "primary" : undefined} disabled={busy} onClick={() => run("answer", o)}>
                {o}
              </button>
            ))}
            {bundles
              .filter((l) => l.approved)
              .map((l) => (
                <ApprovedLook key={l.bundle} slug={d.slug} look={l} keys={toVerify.length === 0 && l === bundles[0]} />
              ))}
          </div>
          <form
            className="own-answer"
            onSubmit={(e) => {
              e.preventDefault();
              if (own.trim()) run("answer", own);
            }}
          >
            <input value={own} onChange={(e) => setOwn(e.target.value)} placeholder={a.options?.length ? "Or answer in your own words" : "Your answer"} disabled={busy} aria-label="Your answer" />
            <button type="submit" className={bundles.length ? undefined : "primary"} disabled={busy || !own.trim()}>
              Answer
            </button>
            {question && (
              <>
                <button type="button" className="ghost" disabled={busy} onClick={() => run("ask-agent")} title="Crew look into it and answer for you">
                  Ask an agent
                </button>
                <button type="button" className="ghost" disabled={busy} onClick={() => run("dismiss")}>
                  Dismiss
                </button>
              </>
            )}
          </form>
        </div>
      );
    }
    case "wait":
      // A Wait on someone else: marked done once it's happened; watched,
      // ws can look now.
      return <WaitActs d={d} busy={busy} run={run} />;
    case "accept":
    case "archive":
    case "unpark":
      return (
        <>
          <button className="primary" disabled={busy} onClick={() => run(a.verb as "accept" | "archive" | "unpark")}>
            {label(a.label)}
          </button>
          {h.kind === "ended" && (
            <button className="ghost" disabled={busy} onClick={() => run("unend")} title="Take the ending back: it's in play again">
              Reopen
            </button>
          )}
        </>
      );
    case "launch-queued":
      return (
        <>
          <button className="primary" disabled={busy} onClick={() => run("launch-queued")}>
            Launch the queued crew
          </button>
          <button className="ghost" disabled={busy} onClick={() => run("dismiss")}>
            Dismiss it
          </button>
        </>
      );
    case "launch":
      return (
        <>
          {a.target && (
            <button className="primary" disabled={busy} onClick={() => run("resume", undefined, a.target)}>
              <Icon name="rotate" /> Resume {a.target}
            </button>
          )}
          <LaunchCrew picks={d.crewPicks} primary={!a.target} again={!!a.target} busy={busy} launch={(p) => run("launch", undefined, undefined, p)} />
        </>
      );
    case "resume":
      return (
        <button className="primary" disabled={busy} onClick={() => run("resume", undefined, a.target)}>
          <Icon name="rotate" /> Resume {a.target}
        </button>
      );
    case "brief":
      return (
        <button className="primary" disabled={busy} onClick={() => run("brief")}>
          <Icon name="pencil" /> Brief it
        </button>
      );
    case "confirm":
      return (
        <>
          <button className="primary" disabled={busy} onClick={() => run("confirm")}>
            <Icon name="play" /> Confirm and start
          </button>
          <Link to={briefHref(d.slug)} className="button-link">
            Read them first
          </Link>
        </>
      );
    case undefined:
    case "":
      return a.label ? <span className="dim">{label(a.label)}</span> : null;
    case "review": {
      const [repo, name] = (a.target ?? "").split("/");
      return (
        <Link to={`/missions/${d.slug}/review/${repo}/${name}`} className="button-link primary" title="Review it (v)">
          <Icon name="rotate" /> Re-review {a.target} <Keys k="v" />
        </Link>
      );
    }
    case "land": {
      // No pull request: it's reviewed, and then landed, on its review.
      const [repo, name] = (a.target ?? "").split("/");
      return (
        <Link to={`/missions/${d.slug}/review/${repo}/${name}`} className="button-link primary" title="Review it, and land it (v)">
          <Icon name="eye" /> {label(a.label)} <Keys k="v" />
        </Link>
      );
    }
    case "review-step": {
      // A Review step: what the steps before made, yours to review. Your
      // review approving what the plan put under review passes the step,
      // as going on does; crew's notes beside it are only to read. Asking
      // for changes (on the review, or here in words) sends it back to the
      // crew who made it.
      const task = a.target ? Number(a.target) || undefined : undefined;
      const looks = a.look ?? [];
      const toApprove = looks.some((l) => !l.read && !l.approved);
      const lead = looks.find((l) => !l.approved) ?? looks[0];
      return (
        <div className="answer">
          <div className="button-row">
            {looks.map((l) => {
              const first = l === lead;
              const primary = first && !l.read && !l.approved;
              if (l.bundle && l.approved) return <ApprovedLook key={l.bundle} slug={d.slug} look={l} keys={first} />;
              if (l.bundle) {
                // A bundle is what's to approve, on the Verify tab.
                return (
                  <Link key={l.bundle} to={bundleLink(d.slug, l.bundle)} className={primary ? "button-link primary" : "button-link"} title="Look at what it shows: approving it passes the step">
                    <Icon name="list-checks" /> Verify {l.name} {first && <Keys k="v" />}
                  </Link>
                );
              }
              if (l.capsule) {
                const [repo, name] = l.capsule.split("/");
                return (
                  <Link key={l.capsule} to={`/missions/${d.slug}/review/${repo}/${name}`} className={primary ? "button-link primary" : "button-link"} title="Review its work: approving it passes the step">
                    <Icon name="eye" /> Review {l.name} {first && <Keys k="v" />}
                  </Link>
                );
              }
              // An image or a video (crew's proof) is seen, not read.
              const media = mediaKind(l.doc ?? l.name ?? "");
              const verb = media === "video" ? "Watch" : media ? "See" : "Read";
              const why = l.read ? "crew's notes on the work, which approving needn't cover" : "approving it passes the step";
              return (
                <Link key={l.doc} to={docLink(d.slug, l.doc ?? "")} className={primary ? "button-link primary" : "button-link"} title={`${verb} it: ${why}`}>
                  <Icon name={media ? "eye" : "file"} /> {verb} {l.name} {first && <Keys k="v" />}
                </Link>
              );
            })}
            <button className={toApprove ? undefined : "primary"} disabled={busy} onClick={() => (task ? run("answer", "Go on") : run("accept"))} title="Pass the step as it is">
              Go on
            </button>
          </div>
          {task && (
            <form
              className="own-answer"
              onSubmit={(e) => {
                e.preventDefault();
                if (own.trim()) run("answer", own);
              }}
            >
              <input value={own} onChange={(e) => setOwn(e.target.value)} placeholder="Or say what's to change, and it goes back to its crew" disabled={busy} aria-label="What's to change" />
              <button type="submit" disabled={busy || !own.trim()}>
                Send back
              </button>
            </form>
          )}
        </div>
      );
    }
    case "review-doc":
      return (
        <Link to={docLink(d.slug, a.target ?? "")} className="button-link primary" title="Read them (v)">
          <Icon name="reply" /> {label(a.label)} <Keys k="v" />
        </Link>
      );
    case "open":
    case "merge":
      return a.url ? (
        <a href={a.url} target="_blank" rel="noopener" className="button-link primary">
          <Icon name="pr" /> {a.verb === "merge" ? `Open ${a.target} to merge it` : `Open ${a.target}`} <Icon name="external" size={12} />
        </a>
      ) : (
        <span className="dim">{label(a.label)}</span>
      );
    case "log":
      return (
        <Link to={`/missions/${d.slug}?tab=log`} className="button-link">
          Read the log
        </Link>
      );
    case "talk":
      // Its planner's conversation is the brief's own page's: what it
      // proposes for the brief and plan is read and confirmed there. What
      // else a conversation proposed (crew, a decision) is confirmed in the
      // conversation, where its cards are.
      return (
        <Link to={a.target && a.target === d.planner && h.kind !== "proposed" ? briefHref(d.slug) : `/missions/${d.slug}/conversations/${a.target}`} className="button-link primary">
          <Icon name="reply" /> {label(a.label)}
        </Link>
      );
    case "reply":
    case "permission": {
      // What crew's asking, answered here; a reply, written here.
      const c = d.crew.find((x) => x.id === a.target && !x.ended);
      if (c && (c.activity === "permission" || c.activity === "question")) return <HeadlinePrompt slug={d.slug} crew={c} />;
      if (c && a.verb === "reply" && c.status !== "asking") return <QuickReply slug={d.slug} id={c.id} reload={reload} />;
      if (c) return <CrewLink slug={d.slug} id={c.id} verb={a.verb} />;
      break;
    }
    case "watch":
      if (d.crew.some((c) => c.id === a.target && !c.ended)) {
        return (
          <Link to={`/missions/${d.slug}/crew/${a.target}`} className="button-link primary">
            <Icon name="message" /> {a.verb === "watch" ? "Watch" : "Open"} {a.target}
          </Link>
        );
      }
  }
  return (
    <span className="dim">{label(a.label)}</span>
  );
}

// EndAct ends the mission: as suggested, with what's suggested shown on
// the card above it, or (nothing suggested) as you say in the End dialog.
function EndAct({ d, reload }: { d: MissionDetail; reload: Reload }) {
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const suggested = d.conclude?.ending;
  const run = (verb: "end" | "dismiss") => {
    setBusy(true);
    act(d.slug, verb)
      .then((r) => {
        if (verb === "end") return ended(d.slug, r.message, reload);
        toast(r.message, { tone: "done" });
        return reload();
      })
      .catch((e: Error) => toast(humanError(e.message), { tone: "danger" }))
      .finally(() => setBusy(false));
  };
  return (
    <>
      {suggested ? (
        <>
          <button className="primary" disabled={busy} onClick={() => run("end")}>
            End it, {endingWords[suggested]}
          </button>
          <button className="ghost" disabled={busy} onClick={() => setOpen(true)}>
            Say how it ends…
          </button>
          {!d.flag && !d.dispatch && (
            <button className="ghost" disabled={busy} onClick={() => run("dismiss")} title="It doesn't end yet: put the suggestion aside">
              Not yet
            </button>
          )}
        </>
      ) : (
        <button className="primary" onClick={() => setOpen(true)}>
          Wrap it up
        </button>
      )}
      {open && <EndDialog d={d} onClose={() => setOpen(false)} reload={reload} />}
    </>
  );
}

// HeadlinePrompt is what crew are stopped on, answered from the mission's
// page: the step it wants permission for (read from its transcript), or
// its question.
function HeadlinePrompt({ slug, crew }: { slug: string; crew: CrewDetail }) {
  const { value: pending } = useLoaded<TranscriptEntry | undefined>(
    crew.activity === "permission" ? () => getTranscript(slug, crew.id).then(pendingCall) : null,
    [slug, crew.id, crew.activity],
    { keep: true },
  );
  return (
    <div className="headline-prompt">
      <PromptCard slug={slug} crew={crew} pending={pending} compact />
      <Link to={`/missions/${slug}/crew/${crew.id}`} className="more-link">
        Open {crew.id}'s conversation
      </Link>
    </div>
  );
}

function CrewLink({ slug, id, verb }: { slug: string; id: string; verb: string }) {
  return (
    <Link to={`/missions/${slug}/crew/${id}`} className="button-link primary">
      <Icon name="message" /> {verb === "watch" ? "Watch" : "Open"} {id}
    </Link>
  );
}

// QuickReply answers crew that are waiting, from the mission's page.
function QuickReply({ slug, id, reload }: { slug: string; id: string; reload: Reload }) {
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const send = () => {
    if (!text.trim()) return;
    setBusy(true);
    sayToCrew(slug, id, text)
      .then(
        (r) => {
          setText("");
          toast(r.message, { tone: "done" });
          return reload();
        },
        (e: Error) => toast(humanError(e.message), { tone: "danger" }),
      )
      .finally(() => setBusy(false));
  };
  return (
    <div className="answer">
      <form
        className="own-answer"
        onSubmit={(e) => {
          e.preventDefault();
          send();
        }}
      >
        <input value={text} onChange={(e) => setText(e.target.value)} placeholder={`Reply to ${id}…`} aria-label={`Reply to ${id}`} disabled={busy} />
        <button type="submit" className="primary" disabled={busy || !text.trim()}>
          Send
        </button>
      </form>
      <Link to={`/missions/${slug}/crew/${id}`} className="more-link">
        Open {id}'s conversation
      </Link>
    </div>
  );
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="summary-row">
      <dt>{label}</dt>
      <dd>{children}</dd>
    </div>
  );
}

// CapsuleCard is a capsule in a line on Summary: its name and branch, where
// it stands, and a way to review it. The Work tab has the rest.
function CapsuleCard({ c, selected, review }: { c: CapsuleDetail; selected: (k: string) => string; review: (c: CapsuleDetail) => void }) {
  const key = `c:${c.repo}/${c.name}`;
  return (
    <div id={`item-${key}`} className={`capsule-card${selected(key)}`}>
      <div className="capsule-line">
        <Icon name="box" size={14} className="dim" />
        <span className="mono capsule-name">
          {c.repo}/{c.name}
        </span>
        {c.branch && c.branch !== c.name && (
          <span className="chip tone-quiet" title="Its branch">
            <Icon name="branch" size={11} />
            {c.branch}
          </span>
        )}
        <CapsuleBadges c={c} />
        <span className="spacer" />
        {c.exists && (
          <button className="small" onClick={() => review(c)} title="Review its changes (v)">
            <Icon name="eye" /> Review
          </button>
        )}
      </div>
    </div>
  );
}

// cameFrom is the conversation a mission came out of: the workspace's (w3),
// or another mission's (cache/t4), on its page.
const cameFrom = (ref: string) => {
  const [slug, id] = ref.includes("/") ? ref.split("/") : ["", ref];
  return talkHref(slug, id);
};

// docLink is a mission's document's page, to read and comment on.
const docLink = (slug: string, ref: string) => `/missions/${slug}/artifact?${new URLSearchParams({ ref })}`;

// bundleLink is where a bundle is looked at: the mission's Verify tab,
// open at it.
// ApprovedLook is the way back to a bundle you've approved, at a step that
// looks at it: it's not what the step waits on.
function ApprovedLook({ slug, look, keys }: { slug: string; look: Look; keys: boolean }) {
  return (
    <Link to={bundleLink(slug, look.bundle!)} className="button-link" title="You approved it: look at what it shows again">
      <Icon name="check-circle" /> {look.name}, approved {keys && <Keys k="v" />}
    </Link>
  );
}

const bundleLink = (slug: string, id: string) => `/missions/${slug}?${new URLSearchParams({ tab: "verify", b: id })}`;

// Brief is the brief, rendered, with its plan under it. They're changed on
// their own page (e, or Edit), where what the planner proposes for them is
// read as tracked changes and its ask bar talks to it.
function Brief({ d, reload }: { d: MissionDetail; reload: Reload }) {
  return (
    <div className="brief">
      {d.request && (
        <Callout tone="quiet" icon="message" title="Requested">
          <Inline text={d.request} />
        </Callout>
      )}
      <div className="brief-tools">
        <Link to={briefHref(d.slug)} className="button-link small-link" title="Edit the brief and plan on their own page, and ask its planner about them (e)">
          <Icon name="pencil" size={12} /> Edit <Keys k="e" />
        </Link>
      </div>
      {d.brief.trim() ? (
        <div className="prose">
          <Markdown text={d.brief} />
        </div>
      ) : (
        <Empty
          icon="file"
          title="No brief yet"
          action={
            <Link to={briefHref(d.slug)} className="button-link">
              Write one
            </Link>
          }
        >
          The brief is what crew work from: the goal, the context, what's in and out, and how you'll know it's done.
        </Empty>
      )}
      <Plan d={d} reload={reload} />
    </div>
  );
}

// Plan is the mission's plan, under its brief: its route from A to Z (what
// Z is, where it may stop early, its steps and who's on each), which ws
// moves it along in place of the brief's stages. It's edited on the brief's
// own page; Ask the planner asks it for a revision.
function Plan({ d, reload }: { d: MissionDetail; reload: Reload }) {
  const [busy, setBusy] = useState(false);
  // Asking the planner for a change: it revises the plan in its
  // conversation, applying it, or proposing it to you there when it
  // touches a checkpoint or Z.
  const [asking, setAsking] = useState(false);
  const [ask, setAsk] = useState("");
  const plan = d.plan ?? "";
  const canAsk = d.state !== "briefing" && !d.ending && d.state !== "archived";
  const askPlanner = () => {
    if (!ask.trim()) return;
    setBusy(true);
    act(d.slug, "replan", { text: ask.trim() })
      .then(
        (r) => {
          toast(r.message, { tone: "done" });
          setAsking(false);
          setAsk("");
          return reload();
        },
        (e: Error) => toast(humanError(e.message), { tone: "danger" }),
      )
      .finally(() => setBusy(false));
  };
  if (!plan.trim()) {
    return null; // no plan: it works from its brief's stages, as the details show
  }
  return (
    <section className="plan">
      <h2 className="group">
        The plan
        <Link to={briefHref(d.slug)} className="button-link small-link" title="Edit the plan on the brief's page">
          <Icon name="pencil" size={12} /> Edit
        </Link>
        {canAsk && !asking && (
          <button className="small" onClick={() => setAsking(true)} title="Ask the planner to revise the plan">
            <Icon name="sparkle" /> Ask the planner
          </button>
        )}
      </h2>
      {asking && (
        <div className="plan-ask">
          <textarea
            autoFocus
            rows={3}
            value={ask}
            onChange={(e) => setAsk(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) askPlanner();
              if (e.key === "Escape") setAsking(false);
            }}
            placeholder="What should change, and why"
            aria-label="What to ask the planner"
          />
          <div className="editor-bar">
            <span className="dim">The steps behind the mission stay as they are. A change to a checkpoint or Z comes back to you to accept; anything else applies.</span>
            <span className="spacer" />
            <button className="ghost" onClick={() => setAsking(false)} disabled={busy}>
              Cancel <Keys k="esc" />
            </button>
            <button className="primary" onClick={askPlanner} disabled={busy || !ask.trim()}>
              Ask <Keys k="mod+⏎" />
            </button>
          </div>
        </div>
      )}
      {d.planProblem && (
        <Callout tone="attention" icon="alert" title="The plan doesn't read, so the mission works from its brief">
          {d.planProblem}
        </Callout>
      )}
      <div className="prose">
        <Markdown text={plan} />
      </div>
    </section>
  );
}

const logFilters = [
  { id: "all", label: "Everything" },
  { id: "you", label: "You" },
  { id: "crew", label: "Crew" },
  { id: "orchestrator", label: "Orchestrator" },
  { id: "threads", label: "Threads" },
] as const;

function Log({ d, selected, openArtifact, go }: Parts) {
  const elsewhere = crewElsewhere(d);
  const [filter, setFilter] = useState<(typeof logFilters)[number]["id"]>("all");
  const [q, setQ] = useState("");
  const events = d.events.filter((e) => {
    const v = viewEvent(e);
    if (filter === "threads" && e.kind !== "thread") return false;
    if (filter !== "all" && filter !== "threads" && v.actor !== filter) return false;
    return !q || plain(e.text).toLowerCase().includes(q.toLowerCase());
  });
  return (
    <>
      <section>
        <div className="log-tools">
          <div className="segmented" role="group" aria-label="Show">
            {logFilters.map((f) => (
              <button key={f.id} aria-pressed={filter === f.id} onClick={() => setFilter(f.id)}>
                {f.label}
              </button>
            ))}
          </div>
          <input type="text" className="search" placeholder="Search the log" value={q} onChange={(e) => setQ(e.target.value)} aria-label="Search the log" />
        </div>
        {events.length ? (
          <Events events={[...events].reverse()} />
        ) : (
          <Empty icon="search" title={d.events.length ? "Nothing matches" : "Nothing yet"}>
            {d.events.length ? "Try another filter or search." : "What happens on the mission shows here as it happens."}
          </Empty>
        )}
      </section>
      {d.crew.length > 0 && (
        <p className="log-crew-moved dim">
          {elsewhere.length < d.crew.length && (
            <>
              Crew are on{" "}
              <button className="link more-link" onClick={() => go("work")}>
                Work
              </button>
              , with the capsule each worked in.{" "}
            </>
          )}
          {elsewhere.length > 0 && "These worked in none of the mission's capsules."}
        </p>
      )}
      {elsewhere.length > 0 && (
        <section>
          <h2 className="group">
            Crew <span className="count">{elsewhere.length}</span>
          </h2>
          {[...elsewhere].reverse().map((c) => (
            <div key={c.id} className="crew-detail">
              <CrewRow c={c} slug={d.slug} detail />
              {c.handoff && (
                <button
                  id={`item-h:${c.id}`}
                  className={`handoff-link${selected(`h:${c.id}`)}`}
                  onClick={() => openArtifact({ ref: c.handoff!, readable: true } as Artifact)}
                >
                  <Icon name="arrow-right" size={13} /> Read {c.id}'s handoff
                </button>
              )}
            </div>
          ))}
        </section>
      )}
    </>
  );
}

// MissionTalk is the mission's Talk tab: conversations about where it's
// going (its plan, what next, what was decided) while crew get on with
// it.
function MissionTalk({ slug, talks, planner }: { slug: string; talks: Talk[]; planner?: string }) {
  const [showResolved, setShowResolved] = useState(false);
  return (
    <div className="mission-talk">
      <TalkList slug={slug} talks={talks} planner={planner} showResolved={showResolved} onToggleResolved={() => setShowResolved((v) => !v)} />
    </div>
  );
}
