import { type ReactNode, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router";
import { act, type Artifact, type CapsuleDetail, getMission, type MissionDetail, useEvents } from "../api";
import { type KeyMap, useKeys, useShortcut } from "../keys";
import { Icon } from "../ui/Icon";
import { keyProps } from "../ui/keyHint";
import { Crumbs } from "../ui/Crumbs";
import { useTitle } from "../ui/useTitle";
import { toast } from "../ui/toast";
import { Skeleton } from "../ui/Skeleton";
import { humanError } from "../ui/text";
import { ErrorCard } from "../ui/ErrorCard";
import { Status } from "../ui/Status";
import { headlineStatus } from "../ui/statusOf";
import { RefContext } from "../refs/context";
import { Brief } from "./Brief/Brief";
import { briefKeys } from "./Brief/words";
import { missionRefs } from "./refs";
import { MissionData } from "./MissionData";
import { isArtifact } from "./outputs";
import { docLink } from "./hrefs";
import { DecisionsTab } from "./DecisionsTab";
import { Verify } from "./Verify/Verify";
import { Work } from "./Work/Work";
import { crewElsewhere, work } from "./workRows";
import { checkRows, rowKey, shownBundle, toApprove, waiting as bundleWaiting } from "./verifyRows";
import { useLoaded } from "../load";
import { PagePin } from "../shell/PagePin";
import { Count } from "../ui/Count";
import { buttonClass } from "../ui/Button";
import { MissionTimeline } from "./MissionTimeline";
import { useTimeline } from "./timeline";
import { PageBar, pageBarTabsClass } from "../ui/PageBar";
import { Tab, Tabs } from "../ui/Tabs";
import { useWorkspace } from "../workspace/store";
import { usePref } from "../prefs";
import { MissionFrame, MissionFrameBody, MissionFrameMain, MissionFramePanel, missionFrameBarClass } from "./Frame/MissionFrame";
import { Log } from "./Log/Log";
import { Side, sideArtifacts } from "./Side/Side";
import { Summary } from "./Summary/Summary";
import { TalkTab } from "./TalkTab";

// The mission page: its time along the top edge, then one bar (back, what
// it's doing, its name, seven tabs) that stays as the page scrolls, and the
// tab under it. Summary leads with what needs you; Brief is the brief and
// the plan, edited as they read, with its planner's conversation beside
// them (Brief/Brief.tsx); Work is its capsules and documents; Verify is its
// verification bundles, what crew handed over to show their work; Log is
// everything that's happened, and its crew; Talk is conversations about it;
// Decisions is what's been decided on it, yours apart, and where a
// decision's name leads (DecisionsTab.tsx).
// The details (state, stages, documents, capsules and crew) are beside
// Summary; on the other tabs, which want the room, s slides them in over
// the page. Summary, Log and Talk are read, in a column; Brief, Verify and
// Work take the page's width.

const tabs = ["summary", "brief", "work", "verify", "log", "talk", "decisions"] as const;
type Tab = (typeof tabs)[number];
const tabNames: Record<Tab, string> = { summary: "Summary", brief: "Brief", work: "Work", verify: "Verify", log: "Log", talk: "Talk", decisions: "Decisions" };
// The tabs that are somewhere to work, not something to read.
const wideTabs: Tab[] = ["work", "verify"];

export const missionKeys: [string, string][] = [
  ["1–7", "Summary, Brief, Work, Verify, Log, Talk, Decisions"],
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
  const strip = useTimeline(d);
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
  // The decision Decisions marks: the one its address names (d).
  const markDecision = (id?: string) => setParams(id ? { tab: "decisions", d: id } : { tab: "decisions" }, { replace: true });

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
    "7": () => setTab("decisions"),
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
    h: back,
  });
  // r reads it again, as Try again does where reading it failed: the
  // button's handed this, and says the key.
  const again = useShortcut("r", load);

  // The bar's ends, the same while the mission loads as once it has: back
  // to the missions, and the tabs (with what's waiting in each, once known).
  const wayBack = { to: "/", title: "Missions", keys: "esc", label: "Back to the missions" };
  const tabRow = (count?: (t: Tab) => ReactNode) => (
    <Tabs className={pageBarTabsClass()} value={tab} onChange={setTab} controls="mission-panel" aria-label="Mission">
      {tabs.map((t) => (
        <Tab key={t} value={t} title={tabNames[t]} keys={String(tabs.indexOf(t) + 1)}>
          {tabNames[t]}
          {count?.(t)}
        </Tab>
      ))}
    </Tabs>
  );

  if (error && !d) {
    return (
      <div className="page narrow">
        <Crumbs steps={[{ to: "/", label: "Missions", keys: "esc" }]} />
        <ErrorCard
          title={/not found|no mission/i.test(error) ? "There's no mission here" : "Couldn't load this mission"}
          error={/not found|no mission/i.test(error) ? `No mission is called ${slug}: it may have been renamed or deleted.` : error}
          retry={again}
        />
        <Link to="/" className={buttonClass()} {...keyProps("esc", "Back to the missions")}>
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
      <MissionFrame>
        <MissionTimeline />
        <PageBar className={missionFrameBarClass} back={wayBack} status={listed && <Status s={headlineStatus(listed.headline)} iconOnly size={16} />} title={listed?.title} tabs={tabRow()} />
        <MissionFrameBody>
          <MissionFrameMain>
            <Skeleton rows={6} />
          </MissionFrameMain>
        </MissionFrameBody>
      </MissionFrame>
    );
  }
  const selected = (key: string) => item?.key === key;
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

  const toWork = () => setTab("work");
  // The details: beside Summary, or over another tab (open while over's set).
  const details = (overTab?: boolean) => (
    <Side d={d} selected={selected} openArtifact={openArtifact} review={review} toWork={toWork} reload={load} updated={updated} about={about} over={overTab} open={overTab && over} />
  );
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
    <MissionFrame fills={tab === "verify" || tab === "work" || tab === "brief"}>
      {strip && <MissionTimeline {...strip} />}
      <PageBar
        className={missionFrameBarClass}
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
        under={tab !== "summary" && details(true)}
      >
        <PagePin r={{ kind: "mission", slug: d.slug }} />
      </PageBar>
      {tab === "brief" ? (
        // The brief has the page under the bar to itself: its panes reach
        // the page's edges, each scrolling by itself.
        <MissionFramePanel id="mission-panel" label={tabNames.brief} brief key="brief">
          {error && <ErrorCard title="Couldn't refresh" error={error} retry={again} />}
          <Brief d={d} root={root} updated={updated} reload={load} onEscape={escape} />
        </MissionFramePanel>
      ) : (
      <MissionFrameBody side={docked} wide={wideTabs.includes(tab)}>
        <MissionFrameMain>
          {error && <ErrorCard title="Couldn't refresh" error={error} retry={again} />}
          <MissionFramePanel id="mission-panel" label={tabNames[tab]} key={tab}>
            {tab === "summary" && <Summary d={d} selected={selected} review={review} full={!side} go={setTab} reload={load} />}
            {tab === "work" && w && (
              <Work d={d} w={w} shown={item?.key} select={setSel} review={review} openArtifact={openArtifact} reload={load} seen={readsSeen} />
            )}
            {tab === "verify" && <Verify d={d} bundle={bundle} pick={pickBundle} sel={item?.key} select={setSel} reload={load} />}
            {tab === "log" && <Log d={d} selected={selected} openArtifact={openArtifact} toWork={toWork} />}
            {tab === "talk" && <TalkTab slug={d.slug} talks={d.talks} planner={d.planner} />}
            {tab === "decisions" && <DecisionsTab d={d} marked={params.get("d") ?? undefined} mark={markDecision} reload={load} />}
          </MissionFramePanel>
        </MissionFrameMain>
        {docked && details()}
      </MissionFrameBody>
      )}
    </MissionFrame>
    </RefContext.Provider>
    </MissionData.Provider>
  );
}
