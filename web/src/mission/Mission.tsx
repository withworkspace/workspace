import { type ReactNode, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router";
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
  type MissionAct,
  type MissionCard,
  type MissionDetail,
  type Talk,
  useEvents,
} from "../api";
import { type KeyMap, useKeys } from "../keys";
import { Markdown } from "../markdown/LazyMarkdown";
import { TalkList, talkHref } from "../talk/Talk";
import { Icon, type IconName } from "../ui/Icon";
import { Crumbs } from "../ui/Crumbs";
import { CheckingNow, checkingTitle } from "./checking";
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
import { BriefTab, briefHref, briefKeys } from "./BriefTab";
import { missionRefs } from "./refs";
import { ArtifactRow, CapsuleBadges, CrewRow, Decisions, Events, isArtifact, MissionData, Stepper, Timeline, viewEvent } from "./parts";
import { EndButton, EndDialog, EndedCapsules, ended, HandedOn, ZChecks } from "./Ending";
import { PlanList } from "./PlanList";
import { waitHow } from "./planRows";
import { Verify } from "./Verify";
import { useRepoLabel, Work, workWords } from "./Work";
import { crewElsewhere, work } from "./workRows";
import { checkRows, rowKey, shownBundle, toApprove, waiting as bundleWaiting } from "./verifyRows";
import { type Reload, useHere, useLoaded } from "../load";
import { PagePin } from "../shell/focus";
import { Chip } from "../ui/Chip";
import { Count } from "../ui/Count";
import { Button, buttonClass } from "../ui/Button";
import { Clamp } from "../ui/Clamp";
import { PageBar, pageBarTabsClass } from "../ui/PageBar";
import { MoreLink } from "../ui/MoreLink";
import { GroupHead } from "../ui/GroupHead";
import { Summary as SummaryList, SummaryRow } from "../ui/Summary";
import { Segment, Segmented } from "../ui/Segmented";
import { cx } from "../ui/cx";
import { Menu, MenuItem } from "../ui/Menu";
import { Tab, Tabs } from "../ui/Tabs";
import { Input } from "../ui/Input";
import { useWorkspace } from "../workspace/store";
import { usePref } from "../prefs";

// The mission page: its time along the top edge, then one bar (back, what
// it's doing, its name, six tabs) that stays as the page scrolls, and the
// tab under it. Summary leads with what needs you; Brief is the brief and
// the plan, edited as they read, with its planner's conversation beside
// them (BriefTab.tsx); Work is its capsules and documents; Verify is its
// verification bundles, what crew handed over to show their work; Log is
// everything that's happened, and its crew; Talk is conversations about it.
// The details (state, stages, documents, capsules and crew) are beside
// Summary; on the other tabs, which want the room, s slides them in over
// the page. Summary, Log and Talk are read, in a column; Brief, Verify and
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
  ["e", "Go to the brief and plan, to edit them"],
  ["s", "Show or hide the details: beside Summary, over the other tabs"],
  ["r", "Reload"],
  ["y / n", "On Verify: approve the check (or mark it looked at) / say it's not right"],
  ["mod+⏎", "On Verify: approve the bundle, or send it back with what you said"],
  ["esc", "Close the details or a bundle you opened, or back to the missions"],
  ...briefKeys,
];


// remembered is each mission's selection, kept while you open what's
// selected and come back.
const remembered = new Map<string, string>();

// Item is something on the page the cursor can be on.
interface Item {
  key: string;
  open: () => void;
  url?: string;
  capsule?: CapsuleDetail;
}

export function MissionPage() {
  const { slug = "" } = useParams();
  // What the workspace's list says of the mission: the page is drawn
  // again when that changes, not when another mission does.
  const inList = useWorkspace((w) => w.missions.find((m) => m.slug === slug));
  const root = useWorkspace((w) => w.root);
  const readsSeen = useWorkspace((w) => w.readsSeen);
  const [params, setParams] = useSearchParams();
  const tab: Tab = tabs.includes(params.get("tab") as Tab) ? (params.get("tab") as Tab) : "summary";
  const navigate = useNavigate();
  const updated = inList?.updated;
  // What it's about, by name (a plugin's thing, a pull request): as the
  // workspace says of it.
  const about = inList?.about;
  // The details beside Summary (kept, as you left them), and over another
  // tab (until you leave it).
  const [side, setSide] = usePref("missionSide");
  const [over, setOver] = useState(false);
  useEffect(() => setOver(false), [tab, slug]);
  const docked = tab === "summary" && side;

  // As the daemon has it, with the capsules' PRs as GitHub last said;
  // again whenever the mission changes (GitHub answering differently, too).
  const mission = useLoaded<MissionDetail>(() => getMission(slug), [slug], { keep: true });
  const { value: d, error, reload: load } = mission;
  // The mission follows its own topic: told at once when its records
  // change, and of its files and capsules at the server's next look. A
  // stream opened again (the page shown again) catches up; the first is
  // what was just loaded.
  const opened = useRef(false);
  useEffect(() => {
    opened.current = false;
  }, [slug]);
  // Where a capsule stands may not have been read when the page first
  // asked (unread): the daemon reads it behind that, and may be done
  // before this stream's open to say so. So the first is read again too.
  const unread = useRef(false);
  unread.current = !!d?.capsules?.some((c) => c.unread);
  useEvents(`mission:${slug}`, {
    hello: () => {
      if (opened.current || unread.current) load();
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
  const toggleSide = () => setSide((v) => !v);
  const back = () => navigate("/");
  // esc closes what's open over the tab, or goes back to the missions.
  const escape = () => {
    if (over && tab !== "summary") return setOver(false);
    // A bundle opened when none is yours to approve closes, to Verify saying so.
    if (tab === "verify" && picked && d && !toApprove(d.bundles).length) return pickBundle();
    back();
  };
  // Brief has its own j, k and esc (its changes to read, its editors):
  // there, the list's aren't bound, and its esc comes here last.
  const listKeys: KeyMap =
    tab === "brief"
      ? {}
      : {
          j: () => move(at + 1),
          ArrowDown: () => move(at + 1),
          k: () => move(at < 0 ? 0 : at - 1),
          ArrowUp: () => move(at < 0 ? 0 : at - 1),
          Escape: escape,
        };
  useKeys({
    "1": () => setTab("summary"),
    "2": () => setTab("brief"),
    "3": () => setTab("work"),
    "4": () => setTab("verify"),
    "5": () => setTab("log"),
    "6": () => setTab("talk"),
    ...listKeys,
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
    e: () => setTab("brief"),
    f: () => {
      const only = d?.capsules.filter((c) => c.exists);
      const c = item?.capsule ?? (only?.length === 1 ? only[0] : undefined);
      if (c) navigate(`/missions/${slug}/files/${c.repo}/${c.name}`);
      else if (only?.length) setTab("work");
    },
    s: () => (tab === "summary" ? toggleSide() : setOver((v) => !v)),
    r: load,
    h: back,
  });

  // The bar's ends, the same while the mission loads as once it has: back
  // to the missions, and the tabs (with what's waiting in each, once known).
  const wayBack = { to: "/", title: "Missions (esc)", label: "Back to the missions" };
  const tabRow = (count?: (t: Tab) => ReactNode) => (
    <Tabs className={pageBarTabsClass()} value={tab} onChange={setTab} controls="mission-panel" aria-label="Mission">
      {tabs.map((t) => (
        <Tab key={t} value={t} title={`${tabNames[t]} (${tabs.indexOf(t) + 1})`}>
          {tabNames[t]}
          {count?.(t)}
        </Tab>
      ))}
    </Tabs>
  );

  if (error && !d) {
    return (
      <div className="page narrow">
        <Crumbs steps={[{ to: "/", label: "Missions" }]} />
        <ErrorCard
          title={/not found|no mission/i.test(error) ? "There's no mission here" : "Couldn't load this mission"}
          error={/not found|no mission/i.test(error) ? `No mission is called ${slug}: it may have been renamed or deleted.` : error}
          retry={load}
        />
        <Link to="/" className={buttonClass()}>
          <Icon name="arrow-left" /> All missions
        </Link>
      </div>
    );
  }
  if (!d) {
    // The page's own frame while it loads, so nothing moves when it has:
    // the bar where it will be, with what the list already says of the
    // mission (where it stands, its name) and the tabs.
    const listed = inList;
    return (
      <div className="mission-page">
        <div className="timeline" aria-hidden>
          <div className="timeline-track" />
        </div>
        <PageBar className="mission-bar" back={wayBack} status={listed && <Status s={headlineStatus(listed.headline)} iconOnly size={16} />} title={listed?.title} tabs={tabRow()} />
        <div className="page mission">
          <div className="mission-main" data-page-scroller="">
            <Skeleton rows={6} />
          </div>
        </div>
      </div>
    );
  }
  const selected = (key: string) => (item?.key === key ? " selected" : "");
  // Your turn in them, or something they proposed to confirm. Its
  // planner's is Brief's, where it's beside the brief and plan.
  const waitingTalks = d.talks.filter((x) => x.id !== d.planner && (x.waiting || !!x.proposed)).length;

  // What its planner has for you on Brief: what it proposed (a draft of
  // the brief and plan, a change to them, anything else), or your turn.
  const planning = d.talks.find((x) => x.id === d.planner);
  const briefWaiting = planning?.proposed || (planning?.waiting ? 1 : 0);

  // Bundles waiting for you, and those nothing has taken the place of:
  // what Verify has to show.
  const waitingBundles = d.bundles.filter(bundleWaiting).length;
  const standing = d.bundles.filter((b) => !b.replacedBy).length;

  const parts = { d, selected, openArtifact, review, go: setTab, reload: load };
  const rename = (to: string) =>
    act(d.slug, "title", { text: to }).then(
      (r) => {
        toast(r.message, { tone: "done" });
        load();
      },
      (e: Error) => toast(humanError(e.message), { tone: "danger" }),
    );
  return (
    <MissionData.Provider value={d}>
    <RefContext.Provider value={refs}>
    <div className={`mission-page${tab === "verify" || tab === "work" || tab === "brief" ? " fills" : ""}`}>
      <Timeline d={d} />
      <PageBar
        className="mission-bar"
        back={wayBack}
        status={d.headlines[0] && <Status s={headlineStatus(d.headlines[0])} iconOnly size={16} />}
        title={d.title}
        onRename={rename}
        renameLabel="Rename the mission"
        tabs={tabRow((t) => (
          <>
            {t === "brief" && briefWaiting > 0 && (
              <Count tone="attention" title={planning?.proposed ? "Its planner proposed something, for you to confirm" : "Its planner's waiting on you"}>
                {briefWaiting}
              </Count>
            )}
            {t === "verify" && waitingBundles > 0 && (
              <Count tone="attention" title="Verification bundles waiting for you to approve">
                {waitingBundles}
              </Count>
            )}
            {t === "verify" && !waitingBundles && standing > 0 && (
              <Count title="Verification bundles, handed over to show the work">
                {standing}
              </Count>
            )}
            {t === "talk" && waitingTalks > 0 && (
              <Count tone="attention" title="Your turn, or something to confirm">
                {waitingTalks}
              </Count>
            )}
          </>
        ))}
        under={tab !== "summary" && <DetailSide {...parts} updated={updated} about={about} over open={over} />}
      >
        <PagePin r={{ kind: "mission", slug: d.slug }} />
      </PageBar>
      {tab === "brief" ? (
        // The brief has the page under the bar to itself: its panes reach
        // the page's edges, each scrolling by itself.
        <div role="tabpanel" id="mission-panel" aria-label={tabNames.brief} className="tab-panel brief-panel" key="brief">
          {error && <ErrorCard title="Couldn't refresh" error={error} retry={load} />}
          <BriefTab d={d} root={root} updated={updated} reload={load} onEscape={escape} />
        </div>
      ) : (
      <div className={`page mission${docked ? " with-side" : ""}${wideTabs.includes(tab) ? " wide" : ""}`}>
        <div className="mission-main" data-page-scroller="">
          {error && <ErrorCard title="Couldn't refresh" error={error} retry={load} />}
          <div role="tabpanel" id="mission-panel" aria-label={tabNames[tab]} className="tab-panel" key={tab}>
            {tab === "summary" && <Summary {...parts} full={!side} />}
            {tab === "work" && w && (
              <Work d={d} w={w} shown={item?.key} select={setSel} review={review} openArtifact={openArtifact} reload={load} seen={readsSeen} />
            )}
            {tab === "verify" && <Verify d={d} bundle={bundle} pick={pickBundle} sel={item?.key} select={setSel} reload={load} />}
            {tab === "log" && <Log {...parts} />}
            {tab === "talk" && <MissionTalk slug={d.slug} talks={d.talks} planner={d.planner} />}
          </div>
        </div>
        {docked && <DetailSide {...parts} updated={updated} about={about} />}
      </div>
      )}
    </div>
    </RefContext.Provider>
    </MissionData.Provider>
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
    <Button variant="ghost" size="small" disabled={busy} onClick={run} title={parked ? "Take it off parked" : "Put it aside until you unpark it"}>
      <Icon name={parked ? "play" : "pause"} size={12} /> {parked ? "Unpark" : "Park"}
    </Button>
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
            <MoreLink className="more-under" onClick={() => go("work")}>
              {d.artifacts.length - sideArtifacts} more in Work
            </MoreLink>
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
          <MoreLink className="more-under" onClick={() => go("work")}>
            {live.length ? `${d.crew.length - live.length} earlier, in Work` : `${d.crew.length} so far, none on it now: in Work`}
          </MoreLink>
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
  const [dismissed, setDismissed] = usePref("dismissed");
  const shown = texts.filter((t) => !dismissed.includes(t.slice(0, 80)));
  if (!shown.length) return null;
  const dismiss = (t: string) => setDismissed((was) => [...was.filter((x) => x !== t.slice(0, 80)), t.slice(0, 80)].slice(-50));
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
              <Button size="small" onClick={() => dismiss(t)}>
                Dismiss
              </Button>
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
  const events = d.events.filter((e) => !isNotice(e.text));
  const recent = full ? summaryRecent : summaryRecentWithSide;
  // The step it's on, and what crew last said: in the lead when there's a
  // headline to lead with, else rows of their own.
  const led = d.headlines.length > 0;
  const step = stepLine(d);
  const said = saidLately(d);
  return (
    <>
      <SummaryLead d={d} reload={reload} />
      <Notices d={d} />
      <SummaryList>
        {goal && (
          <SummaryRow label="Goal">
            <Inline text={goal} />{" "}
            <MoreLink className="more-under" onClick={() => go("brief")}>
              Brief
            </MoreLink>
          </SummaryRow>
        )}
        {!led && !full && step && <SummaryRow label="Now">{step}</SummaryRow>}
        {!led &&
          said.map((c) => (
            <SummaryRow key={c.id} label={`From ${c.id}`}>
              <Said text={c.lastReply!} />
            </SummaryRow>
          ))}
        {full && d.stages.length > 0 && (
          <SummaryRow label={d.plan ? "Plan" : "Stages"}>
            <Stepper stages={d.stages} />
          </SummaryRow>
        )}
        {full && (
          <SummaryRow label="Work">
            {w.hand.map((x) => (
              <CapsuleCard key={x.key} c={x.c} selected={selected} review={review} />
            ))}
            <span className="dim summary-work">
              {workWords(w, label, d.slug) || "Nothing yet: crew lift a capsule when they start."}{" "}
              {(w.capsules.length > 0 || w.outputs.length > 0) && (
                <MoreLink onClick={() => go("work")}>
                  All of it in Work
                </MoreLink>
              )}
            </span>
          </SummaryRow>
        )}
        {full && live.length > 0 && (
          <SummaryRow label="Crew">
            {live.map((c) => (
              <CrewRow key={c.id} c={c} slug={d.slug} />
            ))}
          </SummaryRow>
        )}
        {d.decisions.some((x) => x.state === "current") && (
          <SummaryRow label="Decided">
            <Decisions list={d.decisions} limit={summaryDecisions} />
          </SummaryRow>
        )}
        {events.length > 0 && (
          <SummaryRow label="Recently">
            <Events events={events.slice(-recent)} lines={2} />
            {events.length > recent && (
              <MoreLink className="more-under" onClick={() => go("log")}>
                The whole log ({events.length})
              </MoreLink>
            )}
          </SummaryRow>
        )}
        {d.from && (
          <SummaryRow label="Raised from">
            <span className="dim">{d.from}</span>
          </SummaryRow>
        )}
      </SummaryList>
    </>
  );
}

// stepLine is the step the mission's on, as its lead says it over what's
// happening: which it is, and for a Wait, whether anything's watching.
function stepLine(d: MissionDetail): React.ReactNode {
  const now = d.stages.find((s) => s.state === "current");
  return (
    now && (
      <>
        <span title={now.text}>
          {d.plan ? "Step" : "Stage"} {now.n} of {d.stages.length}: <strong>{now.title}</strong>
        </span>
        {now.checkIn && <span className="dim"> · check in after</span>}
        {now.who === "you" && !waitHow(now) && <span className="dim"> · yours: only you pass it</span>}
        {waitHow(now) && (
          <span className="lead-wait">
            {" "}
            · <Chip tone={waitHow(now)!.auto ? "progress" : "attention"}>{waitHow(now)!.word}</Chip> <span className="dim">{waitHow(now)!.says}</span>
          </span>
        )}
      </>
    )
  );
}

// saidLately is the crew still there who've said something and aren't at
// work: their last word is shown with where the mission stands.
const saidLately = (d: MissionDetail) => d.crew.filter((c) => !c.ended && c.lastReply && c.activity !== "working");

// SummaryLead is where the mission stands, as its Summary leads with it:
// the headlines, the first with the step it's on and what crew last said.
// It's drawn alone by the stories (Banners.stories.tsx), which is why it's
// a part of its own.
export function SummaryLead({ d, reload }: { d: MissionDetail; reload: Reload }) {
  const said = saidLately(d);
  return (
    <Cards
      d={d}
      reload={reload}
      step={stepLine(d)}
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
  );
}

// Said is what crew last said, rendered, a few lines of it until you ask
// for the rest.
function Said({ text }: { text: string }) {
  return (
    <Clamp className="said-block" of={text}>
      <Markdown text={text} />
    </Clamp>
  );
}

// Cards lead the Summary: where the mission stands, as ws says it's drawn
// (mission.Cards): the first as the page's subject, with the step it's on
// and what crew last said, and anything else under it as a notice. One
// card, while things are as they should be.
function Cards({ d, reload, step, latest }: { d: MissionDetail; reload: Reload; step?: React.ReactNode; latest?: React.ReactNode }) {
  return (
    <>
      {d.cards.map((c, i) => (
        <CardView key={`${c.kind}-${c.by ?? ""}-${i}`} d={d} c={c} lead={i === 0} reload={reload} step={i === 0 ? step : undefined} latest={i === 0 ? latest : undefined} />
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

// A card's tone is whose it is: yours, failing (and yours), an agent's,
// outside's, done, or nobody's in particular.
const cardTone: Record<MissionCard["tone"], CalloutTone> = { you: "attention", failing: "danger", agent: "progress", outside: "info", done: "done", quiet: "quiet" };

// CardView draws a card as it's given: its title, its sentence, and what
// it offers. Nothing about how is worked out here.
function CardView({ d, c, lead, reload, step, latest }: { d: MissionDetail; c: MissionCard; lead: boolean; reload: Reload; step?: React.ReactNode; latest?: React.ReactNode }) {
  const st = headlineStatus({ kind: c.kind, court: c.court, label: "" } as Headline);
  const title = c.checking ? checkingTitle(c.checking as unknown as NonNullable<Headline["checking"]>) : c.title;
  const detail = c.checking ? undefined : c.detail;
  const long = !!detail && detail.length > 220;
  const offers = !!c.primary || !!c.more?.length;
  const actions = offers ? <CardActs d={d} c={c} lead={lead} reload={reload} /> : undefined;
  const body = (
    <>
      {c.checking && <CheckingNow c={c.checking as unknown as NonNullable<Headline["checking"]>} />}
      {detail &&
        (c.kind === "resume" || (long && !lead) ? (
          <details className="fold">
            <summary>{c.kind === "resume" ? `What ${c.primary?.target ?? "crew"} will be told` : "Read it all"}</summary>
            <Inline text={detail} />
          </details>
        ) : long ? (
          <Clamp lines={3} more="More" less="Less" of={detail}>
            <Inline text={detail} />
          </Clamp>
        ) : (
          <Inline text={detail} />
        ))}
      {c.nothing && <div className="dim card-nothing">Nothing for you to do</div>}
      {lead && (c.kind === "to-brief" || c.kind === "briefing") && d.request && <div className="dim request">Requested: {d.request}</div>}
      {c.kind === "wrap-up" && d.conclude?.ending && (
        <>
          <ZChecks c={d.conclude} />
          <HandedOn to={d.conclude.to} />
        </>
      )}
      {c.kind === "ended" && d.ending && (
        <>
          <ZChecks c={d.ending} />
          <HandedOn to={d.ending.to} />
        </>
      )}
    </>
  );
  // The first leads the page; the rest are notices under it.
  return lead ? (
    <Lead tone={cardTone[c.tone]} icon={st.icon} live={st.live} title={title} step={step} actions={actions} latest={latest}>
      {body}
    </Lead>
  ) : (
    <Callout tone="quiet" icon={st.icon} title={title} className="headline also" actions={actions}>
      {body}
    </Callout>
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
    <Button variant={primary ? "primary" : undefined} disabled={busy} onClick={() => launch()} title={picks && others.length ? `On ${pickName(picks.default)}` : undefined}>
      <Icon name="play" /> Launch {again ? "new " : ""}crew
    </Button>
  );
  if (!picks || !others.length) return main;
  const all = [picks.default, ...others];
  // data-field: keys typed in the menu are its own, not the page's (keys.ts).
  return (
    <div className="split-button">
      {main}
      <Menu
        placement="bottom end"
        aria-label="Launch crew on"
        onAction={(k) => launch(k === "0" ? undefined : all[Number(k)])}
        trigger={
          <Button variant={primary ? "primary" : undefined} className={cx("split-toggle", primary && "split-toggle-primary")} disabled={busy} aria-label="Launch crew on another agent or model" data-field>
            <Icon name="chevron-down" size={13} />
          </Button>
        }
      >
        {all.map((p, i) => (
          <MenuItem key={pickName(p)} id={String(i)} textValue={pickName(p)}>
            <span className="split-pick-name mono">
              {p.harness}
              {p.model ? ` · ${p.model}` : ""}
            </span>
            {!p.model && <span className="dim">its own model</span>}
            {i === 0 && <Chip>default</Chip>}
          </MenuItem>
        ))}
      </Menu>
    </div>
  );
}

// What putting a card aside does is done to the first question a mission
// has, whichever card says it: only the lead offers these.
const leadOnly = new Set(["dismiss", "ask-agent", "not-yet"]);

// actHref is where an act that opens something goes, in ws.
function actHref(d: MissionDetail, c: MissionCard, a: MissionAct): string | undefined {
  const t = a.target ?? "";
  switch (a.do) {
    case "crew":
      return `/missions/${d.slug}/crew/${t}`;
    case "review": {
      const [repo, name] = t.split("/");
      return `/missions/${d.slug}/review/${repo}/${name}`;
    }
    case "verify":
      return bundleLink(d.slug, t);
    case "read":
      return docLink(d.slug, t);
    case "talk":
      // Its planner's conversation is the brief's own page's: what it
      // proposes for the brief and plan is read and confirmed there. What
      // else a conversation proposed is confirmed in the conversation.
      return t && t === d.planner && c.kind !== "proposed" ? briefHref(d.slug) : `/missions/${d.slug}/conversations/${t}`;
    case "brief-page":
      return briefHref(d.slug);
    case "log":
      return `/missions/${d.slug}?tab=log`;
  }
  return undefined;
}

// CardActs is what a card offers, in the order it's given: the one meant
// to move it first, accented, then the rest, the quiet ones quiet. An act
// that takes words has its field beside it when it's the card's first, and
// opens it when pressed otherwise. How each is done is here; which there
// are, and in what order, is ws's to say (mission.Cards).
function CardActs({ d, c, lead, reload }: { d: MissionDetail; c: MissionCard; lead: boolean; reload: Reload }) {
  const navigate = useNavigate();
  const here = useHere();
  const [busy, setBusy] = useState(false);
  const [words, setWords] = useState("");
  const [opened, setOpened] = useState<MissionAct | undefined>();
  const [how, setHow] = useState(false);
  const send = (verb: ActVerb, body: { text?: string; crew?: string; task?: number; harness?: string; model?: string } = {}) => {
    setBusy(true);
    // Archiving asks first when it would burn work that hasn't merged.
    (verb === "archive" ? archiveMission(d.slug) : act(d.slug, verb, body))
      .then((r) => {
        if (!r) return;
        if (verb === "end") return ended(d.slug, r.message, reload);
        // Where it takes you (briefing: its own page) is for the card you
        // pressed it on: gone somewhere else since, you're told instead.
        if (r.path && here()) return navigate(r.path);
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
  // run does an act, with what was written in its field, if it has one.
  const run = (a: MissionAct, written = "") => {
    const said = written.trim();
    switch (a.do) {
      case "answer": {
        const text = said ? `${a.prefix ?? ""}${said}` : a.text;
        if (text) send("answer", { text, task: a.task || undefined });
        return;
      }
      case "accept":
        return send("accept", { text: said });
      case "resume":
        return send("resume", { crew: a.target });
      case "not-yet":
        return send("dismiss");
      case "end-how":
        return setHow(true);
      default:
        return send(a.do as ActVerb);
    }
  };
  const acts = [...(c.primary ? [c.primary] : []), ...(c.more ?? [])].filter((a) => lead || !leadOnly.has(a.do));
  const first = c.primary && acts[0] === c.primary ? c.primary : undefined;
  // Crew stopped on something are answered where they are: what they're
  // asking, here; a reply, written here.
  if (first && (first.do === "reply" || first.do === "permission")) {
    const crew = d.crew.find((x) => x.id === first.target && !x.ended);
    if (crew && (crew.activity === "permission" || crew.activity === "question")) return <HeadlinePrompt slug={d.slug} crew={crew} />;
    if (crew && first.do === "reply" && crew.status !== "asking") return <QuickReply slug={d.slug} id={crew.id} reload={reload} />;
    if (crew) return <CrewLink slug={d.slug} id={crew.id} verb={first.do} />;
    return null;
  }
  const one = (a: MissionAct, i: number) => {
    const accent = a === first;
    // The first act is the one accented, where the card leads the page: a
    // notice's first reads as an ordinary button.
    const variant = accent ? (lead ? "primary" : undefined) : a.quiet ? "ghost" : undefined;
    const inside = (
      <>
        {a.icon && <Icon name={a.icon as IconName} />} {a.label} {lead && a.key && <Keys k={a.key} />}
      </>
    );
    if (a.do === "launch") return <LaunchCrew key={i} picks={d.crewPicks} primary={accent && lead} again={!accent} busy={busy} launch={(p) => send("launch", { harness: p?.harness, model: p?.model })} />;
    if (a.do === "url") {
      // GitHub hasn't said where it is yet: its words, with nothing to open.
      if (!a.url) {
        return (
          <span key={i} className="dim">
            {a.label}
          </span>
        );
      }
      return (
        <a key={i} href={a.url} target="_blank" rel="noopener" className={buttonClass({ variant: accent && lead ? "primary" : undefined })}>
          {inside} <Icon name="external" size={12} />
        </a>
      );
    }
    const href = actHref(d, c, a);
    if (href) {
      return (
        <Link key={i} to={href} className={buttonClass({ variant })}>
          {inside}
        </Link>
      );
    }
    // One that takes words opens its field, but the card's first, whose
    // field is beside it.
    if (a.field && !accent) {
      return opened === a ? null : (
        <Button
          key={i}
          variant={variant}
          disabled={busy}
          onClick={() => {
            setWords("");
            setOpened(a);
          }}
        >
          {inside}
        </Button>
      );
    }
    return (
      <Button key={i} variant={variant} type={accent && a.field ? "submit" : "button"} disabled={busy || (accent && !!a.field && !a.text && a.do === "answer" && !words.trim() && !opened)} onClick={accent && a.field ? undefined : () => run(a)}>
        {inside}
      </Button>
    );
  };
  // Words are all the first asks for: the field, then its button.
  const wordsOnly = !!first?.field && first.do === "answer" && !first.text;
  const field = (a: MissionAct, focus: boolean) => (
    <Input value={words} onChange={(e) => setWords(e.target.value)} placeholder={a.field} disabled={busy} aria-label={a.field} autoFocus={focus} />
  );
  return (
    <div className="answer">
      {first?.field && !opened ? (
        <>
          <form
            className="own-answer"
            onSubmit={(e) => {
              e.preventDefault();
              run(first, words);
            }}
          >
            {wordsOnly && field(first, false)}
            {one(first, 0)}
            {!wordsOnly && field(first, false)}
          </form>
          {acts.length > 1 && <div className="button-row">{acts.slice(1).map((a, i) => one(a, i + 1))}</div>}
        </>
      ) : (
        <div className="button-row">{acts.map((a, i) => (a === first && a.field ? <Button variant="primary" key={i} disabled={busy} onClick={() => setOpened(undefined)}>{a.label}…</Button> : one(a, i)))}</div>
      )}
      {opened && (
        <form
          className="own-answer"
          onSubmit={(e) => {
            e.preventDefault();
            run(opened, words);
          }}
        >
          {field(opened, true)}
          <Button type="submit" disabled={busy || (opened.do === "answer" && !opened.text && !words.trim())}>
            {opened.label.replace(/…$/, "")}
          </Button>
        </form>
      )}
      {how && <EndDialog d={d} onClose={() => setHow(false)} reload={reload} />}
    </div>
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
      <MoreLink to={`/missions/${slug}/crew/${crew.id}`}>
        Open {crew.id}'s conversation
      </MoreLink>
    </div>
  );
}

function CrewLink({ slug, id, verb }: { slug: string; id: string; verb: string }) {
  return (
    <Link to={`/missions/${slug}/crew/${id}`} className={buttonClass({ variant: "primary" })}>
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
        <Input value={text} onChange={(e) => setText(e.target.value)} placeholder={`Reply to ${id}…`} aria-label={`Reply to ${id}`} disabled={busy} />
        <Button variant="primary" type="submit" disabled={busy || !text.trim()}>
          Send
        </Button>
      </form>
      <MoreLink to={`/missions/${slug}/crew/${id}`}>
        Open {id}'s conversation
      </MoreLink>
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
          <Chip tone="quiet" title="Its branch">
            <Icon name="branch" size={11} />
            {c.branch}
          </Chip>
        )}
        <CapsuleBadges c={c} />
        <span className="spacer" />
        {c.exists && (
          <Button size="small" onClick={() => review(c)} title="Review its changes (v)">
            <Icon name="eye" /> Review
          </Button>
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
const bundleLink = (slug: string, id: string) => `/missions/${slug}?${new URLSearchParams({ tab: "verify", b: id })}`;

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
          <Segmented value={filter} onChange={setFilter} aria-label="Show">
            {logFilters.map((f) => (
              <Segment key={f.id} value={f.id}>
                {f.label}
              </Segment>
            ))}
          </Segmented>
          <Input type="text" className="search" placeholder="Search the log" value={q} onChange={(e) => setQ(e.target.value)} aria-label="Search the log" />
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
              <MoreLink onClick={() => go("work")}>
                Work
              </MoreLink>
              , with the capsule each worked in.{" "}
            </>
          )}
          {elsewhere.length > 0 && "These worked in none of the mission's capsules."}
        </p>
      )}
      {elsewhere.length > 0 && (
        <section>
          <GroupHead count={elsewhere.length}>
            Crew
          </GroupHead>
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
