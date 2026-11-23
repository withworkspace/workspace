import { useCallback, useEffect, useLayoutEffect, useMemo, useState, memo } from "react";
import { Link, Navigate, Route, Routes, useLocation, useMatch, useNavigate, useNavigationType, useParams } from "react-router";
import { type Connection, NotSignedIn, setSettings } from "./api";
import { ArtifactPage as ArtifactView, ArtifactsPage, artifactKeys as artifactViewKeys, artifactsKeys } from "./artifacts/Artifacts";
import { ComparePage, compareKeys } from "./artifacts/Compare";
import { FoundArtifactPage, foundArtifactKeys } from "./artifacts/Found";
import { Explorer, explorerKeys } from "./files/Explorer";
import { docKeys } from "./files/DocPage";
import { FilesPage, filesKeys } from "./files/Files";
import { DocumentsPage, documentsKeys, SpacePage, spaceKeys } from "./documents/Documents";
import { codeSearchKey, isCodeSearchKey, leaderWithin, pageBinds, useKeys } from "./keys";
import { ArtifactPage, artifactKeys } from "./mission/ArtifactPage";
import { briefHref } from "./mission/BriefTab";
import { CrewPage, crewKeys } from "./mission/CrewPage";
import { listKeys, MissionList } from "./mission/List";
import { MissionPage, missionKeys } from "./mission/Mission";
import { OrchestratorPage, OrchestratorRunPage } from "./orchestrator/Orchestrator";
import { AppPage } from "./apps/AppPage";
import { RunsPage } from "./runs/Runs";
import { useRefs } from "./refs/known";
import type { Known } from "./refs/refs";
import { hereAt } from "./shell/here";
import { PluginPage } from "./plugins/Plugin";
import { RefsProvider } from "./refs/known";
import { SettingsPage, settingsKeys } from "./settings/Settings";
import { pageKeys } from "./pages/Pages";
import { DocsAddress, DocumentAt, LocalPageAt, OpenAddress, PageAddress } from "./documents/DocumentAt";
import { Review, reviewKeys } from "./review/Review";
import { PRReviewPage, prReviewKeys } from "./reviews/PRReview";
import { ReviewsPage, reviewsKeys } from "./reviews/Reviews";
import { repoOfPage, scopePrefix } from "./shell/find";
import { Palette } from "./shell/Palette";
import { useFaviconBadge, useNotifications } from "./shell/notify";
import { Arrange } from "./shell/Arrange";
import { askShutDown } from "./shell/daemon";
import { arrange, type Arrangement, type Arranged, type Place, placesOf, sameArrangement, saving } from "./shell/places";
import { FocusKeys, FocusSection } from "./shell/focus";
import { noteVisit, visitAt } from "./shell/recent";
import { wantsRoom } from "./shell/room";
import { shortVersion } from "./shell/version";
import { WhatsNew } from "./shell/WhatsNew";
import { useThingMenu } from "./shell/ThingMenu";
import { TalkPage, TalksPage, talkKeys, talksKeys } from "./talk/Talk";
import { TeamPage, teamKeys } from "./team/Team";
import { Banner } from "./ui/Banner";
import { Icon } from "./ui/Icon";
import { Spinner } from "./ui/Spinner";
import { Logotype } from "./ui/Logotype";
import { Viewer } from "./ui/Viewer";
import { useMedia } from "./ui/useMedia";
import { setTitleContext, useTitle } from "./ui/useTitle";
import { toast } from "./ui/toast";
import { Toasts } from "./ui/Toasts";
import { Loading } from "./ui/Loading";
import { Empty } from "./ui/Empty";
import { humanError } from "./ui/text";
import { Keys } from "./ui/Keys";
import { Confirmer } from "./ui/Confirm";
import { Dialog } from "./ui/Dialog";
import { duration } from "./ui/when";
import { SiloPoint } from "./silo/SiloPoint";
import { Count } from "./ui/Count";
import { buttonClass } from "./ui/Button";
import { Below } from "./chrome/Below";
import { useBelowViews } from "./chrome/belowViews";
import { Beside } from "./chrome/Beside";
import { hideSidebar, narrowQuery, sidebarPlace, toggleBelow, toggleBeside } from "./chrome/chromeState";
import { FindButton } from "./chrome/FindButton";
import { Foot } from "./chrome/Foot";
import { FootBelow } from "./chrome/FootBelow";
import { FootBeside } from "./chrome/FootBeside";
import { FootCrew } from "./chrome/FootCrew";
import { FootSidebar } from "./chrome/FootSidebar";
import { FootSep } from "./chrome/FootSep";
import { type Need, useArrived, waitingOf } from "./chrome/need";
import { Needs } from "./chrome/Needs";
import { NotifyToggle } from "./chrome/NotifyToggle";
import { Places } from "./chrome/Places";
import { Shell } from "./chrome/Shell";
import { Sidebar } from "./chrome/Sidebar";
import { useBeside, useChromeStore, useFrameDim, useFrameExpanded, useFrameHere } from "./chrome/store";
import { WorkspaceName } from "./chrome/WorkspaceName";
import { pageScroller } from "./ui/scroller";
import { Menu, MenuItem, MenuSection, MenuSeparator, MenuText } from "./ui/Menu";
import { useFollowWorkspace, usePlugins, useWorkspace, useWorkspaceIfRead, useWorkspaceStore } from "./workspace/store";

// Nothing needing you, before the workspace has said: the same list each
// time, so what's worked out from it isn't worked out again.
const noNeeds: Need[] = [];

export function App() {
  // The workspace is followed here, once, into its store
  // (workspace/store.ts), which everything reads it from. On a plugin's
  // page, ws is told whose app is on screen.
  const onPlugin = useMatch("/x/:name/*")?.params.name;
  useFollowWorkspace(onPlugin);
  const workspace = useWorkspaceIfRead();
  const error = useWorkspaceStore((s) => s.error);
  const connection = useWorkspaceStore((s) => s.connection);
  const plugins = usePlugins();
  // What's open over the window is the window's (chrome/store.ts), so
  // anything can open it: Find (with what it starts with typed), the
  // keys, the sidebar as a drawer, the sidebar being arranged; and
  // whether the daemon was shut down from here, and this page with it.
  const help = useChromeStore((c) => c.help);
  const palette = useChromeStore((c) => c.find);
  const drawer = useChromeStore((c) => c.drawer);
  const arranging = useChromeStore((c) => c.arranging);
  const closed = useChromeStore((c) => c.closed);
  const { openFind, closeFind, setHelp, setDrawer, stopArranging } = useChromeStore.getState();
  const location = useLocation();
  const navigate = useNavigate();
  // On a page that wants the room the sidebar folds to a rail, which opens
  // over the page on hover: one of ws's own (room.ts), or a plugin's page
  // that asks for the chrome expanded (ws:chrome-layout), while ws is on
  // that plugin's pages.
  const expandedFor = useFrameExpanded();
  const room = (wantsRoom(location.pathname) || (expandedFor !== undefined && expandedFor === onPlugin)) && !arranging;
  // How this window is arranged, kept in the browser: whether you've
  // hidden the sidebar. On a narrow window it's a drawer whatever else.
  // It's the window's (chrome/store.ts), kept for the workspace: until the
  // workspace is known it's the arrangement of the one last open here,
  // which is nearly always the same one.
  const chrome = useChromeStore((c) => c.chrome);
  const changeChrome = useChromeStore((c) => c.change);
  const setRoot = useChromeStore((c) => c.setRoot);
  useLayoutEffect(() => setRoot(workspace?.root), [setRoot, workspace?.root]);
  const narrow = useMedia(narrowQuery);
  const place = sidebarPlace(chrome, room, narrow);
  // Beside: a peek at a second thing, next to the page. What you ask
  // about a page opens there (ask/AskBar.tsx), about that page; \ opens
  // and closes it.
  const beside = useBeside();
  useKeys({ "\\": () => beside.all.length > 0 && changeChrome(toggleBeside) });
  // Below, under the page: there while it has something to show, which
  // it hasn't yet (belowViews.ts). It has no key yet either.
  const belowViews = useBelowViews();
  const belowShown = chrome.below.open && belowViews.length > 0;

  // The drawer (on a narrow window) closes on going somewhere.
  useEffect(() => setDrawer(false), [setDrawer, location.pathname]);
  // Arranging ends, unsaved, on going somewhere or leaving the drawer.
  useEffect(() => stopArranging(), [stopArranging, location.pathname]);

  // Where you go is kept, for Find to offer before anything's typed.
  const root = workspace?.root;
  useEffect(() => {
    if (!workspace) return;
    const visit = visitAt(location.pathname, location.search, location.hash, workspace, Date.now());
    if (visit) noteVisit(workspace.root, visit);
    // Only on going somewhere: not each time the workspace changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [location.pathname, location.search, root]);

  // A plugin's app has a backdrop up: what's beside its frame dims with it.
  const dim = useFrameDim();
  const frameHere = useFrameHere();

  useGlobalKeys();
  useKeyboarding();
  useScrollMemory();

  // What needs you: the sidebar's list, and what counts the places, tells
  // the desktop and marks the tab.
  const needs = workspace?.needs ?? noNeeds;
  const rowMenu = useThingMenu();
  const waiting = useMemo(() => waitingOf(needs), [needs]);
  const arrived = useArrived(workspace ? needs : undefined);
  // Who's at work now, for the foot: a window on to the work.
  const atWork = useMemo(() => (workspace?.missions ?? []).flatMap((m) => m.crew.filter((c) => !c.ended && c.activity === "working").map((c) => ({ m, c }))), [workspace]);
  useEffect(() => setTitleContext(workspace?.name ?? "ws", waiting.length), [workspace?.name, waiting.length]);
  useFaviconBadge(waiting.length > 0);
  const openPath = useCallback((p: string) => navigate(p), [navigate]);
  const notify = useNotifications(waiting, openPath);

  const places = useMemo(() => placesOf(workspace, waiting, plugins), [workspace, waiting, plugins]);
  // What's just been arranged, until the workspace says it back.
  const [arranged, setArranged] = useState<Arrangement>();
  const saved = workspace?.places;
  const savedKey = saved && JSON.stringify(saved);
  useEffect(() => setArranged(undefined), [savedKey]);
  const { sidebar, menu } = arrange(places, arranged ?? saved);
  const saveArrangement = async (a: Arranged) => {
    stopArranging();
    const next = saving(a, places, saved);
    if (sameArrangement(next, saving(arrange(places, saved), places, saved))) return;
    setArranged(next);
    try {
      await setSettings([
        { section: "ui", key: "sidebar", value: next.sidebar },
        { section: "ui", key: "menu", value: next.menu },
      ]);
    } catch (e) {
      setArranged(undefined);
      toast(`Couldn't keep the sidebar as arranged: ${humanError(String(e))}`, { tone: "danger" });
    }
  };
  const resetArrangement = async () => {
    stopArranging();
    try {
      await setSettings([
        { section: "ui", key: "sidebar", clear: true },
        { section: "ui", key: "menu", clear: true },
      ]);
    } catch (e) {
      toast(`Couldn't reset the sidebar: ${humanError(String(e))}`, { tone: "danger" });
    }
  };
  // The sidebar's toggle, at the foot's start, which says how many need
  // you while the sidebar's out of sight: on a narrow window it opens the
  // drawer, elsewhere it hides and shows the sidebar.
  const out = place === "hidden" || (place === "drawer" && !drawer);
  const toggleSidebar = () => (narrow ? setDrawer((d) => !d) : changeChrome((c) => hideSidebar(c, !c.hidden)));

  if (closed) {
    return (
      <Notice title="The daemon is closed">
        Its crew were hung up and its web UI went with it. Run <code>ws web</code> in the workspace to start it again.
      </Notice>
    );
  }
  if (error instanceof NotSignedIn) {
    return (
      <Notice title="Not signed in">
        This page signs in through a link from your terminal. Run <code>ws web</code> in the workspace.
      </Notice>
    );
  }
  return (
    <Shell
      sidebar={place}
      drawerOpen={drawer}
      onDrawerClose={() => {
        setDrawer(false);
        stopArranging();
      }}
      dim={dim}
      beside={beside.shown && workspace && <Beside sheet={narrow} />}
      besideOver={beside.over}
      besideSize={beside.width}
      below={belowShown && workspace && <Below views={belowViews} sheet={narrow} />}
      belowSize={chrome.below.height}
      still={!workspace}
      pageClassName={connection === "lost" && workspace ? "stale" : undefined}
      side={
        <Sidebar rail={room && !narrow}>
          <WorkspaceMenu places={menu} />
          <FindButton onPress={() => openFind()} />
          {arranging ? (
            <Arrange start={{ sidebar, menu }} canReset={!!saved} onDone={saveArrangement} onCancel={stopArranging} onReset={resetArrangement} />
          ) : (
            <Places places={sidebar} path={location.pathname} />
          )}
          {workspace && !arranging && <FocusSection path={location.pathname} />}
          {!arranging && <Needs needs={needs} path={location.pathname} fresh={arrived} most={12} menu={rowMenu} tool={notify.supported && <NotifyToggle on={notify.on} onToggle={notify.toggle} />} />}
        </Sidebar>
      }
      foot={
        <Foot
          start={
            <>
              <FootSidebar shown={!out} count={needs.length} fresh={arrived.size > 0} onPress={toggleSidebar} />
              {atWork.length > 0 && <FootSep />}
              {atWork.map(({ m, c }) => (
                <FootCrew key={`${m.slug}:${c.id}`} id={c.id} mission={m.title} to={`/missions/${m.slug}/crew/${c.id}`} />
              ))}
            </>
          }
          // The docks' toggles, in the order they're in the window. Below's
          // is there while it has something to show. Beside's is there
          // while there's something to peek at here; closed, it's marked.
          end={
            (belowViews.length > 0 || beside.all.length > 0) && (
              <>
                {belowViews.length > 0 && <FootBelow open={belowShown} onPress={() => changeChrome(toggleBelow)} />}
                {beside.all.length > 0 && <FootBeside open={beside.shown} waiting={beside.all.length} onPress={() => changeChrome(toggleBeside)} />}
              </>
            )
          }
        />
      }
      overlays={
        <>
          {help && <Help keys={helpKeys(location.pathname)} onClose={() => setHelp(false)} />}
          {palette !== undefined && workspace && (
            <Palette
             
              waiting={waiting}
              start={palette}
              pageRepo={repoOfPage(location.pathname, workspace.spaces)}
              path={location.pathname}
              search={location.search}
              frameHere={frameHere}
              onClose={closeFind}
            />
          )}
          <Viewer />
          <Toasts />
          <Confirmer />
          <WhatsNew version={workspace?.version} />
          <FocusKeys />
        </>
      }
    >
      <ConnectionBanner connection={connection} />
      {error && <Banner className="app-error">{error.message}</Banner>}
      {!workspace && !error && <Loading what="Opening the workspace…" />}
      {workspace && (
        <RefsProvider>
          <SaysHere />
          <Pages notify={notify} />
        </RefsProvider>
      )}
    </Shell>
  );
}

// Pages is the page at the address. It's given nothing of the workspace
// (each page reads what it needs from the store), so it's drawn again on
// going somewhere, and not each time the shell round it is.
const Pages = memo(function Pages({ notify }: { notify: ReturnType<typeof useNotifications> }) {
  const missions = useWorkspace((w) => w.missions);
  return (
    <Routes>
      <Route path="/" element={<MissionList missions={missions} />} />
      <Route path="/missions/:slug" element={<MissionPage />} />
      <Route path="/missions/:slug/artifact" element={<ArtifactPage />} />
      <Route path="/missions/:slug/brief" element={<ToBrief />} />
      <Route path="/missions/:slug/crew/:id" element={<CrewPage />} />
      <Route path="/missions/:slug/orchestrator/:id" element={<OrchestratorRunPage />} />
      <Route path="/orchestrator" element={<OrchestratorPage />} />
      <Route path="/runs" element={<RunsPage />} />
      <Route path="/apps/:name" element={<AppPage />} />
      <Route path="/plugins" element={<Navigate to="/settings/plugins" replace />} />
      <Route path="/settings" element={<SettingsPage notify={notify} />} />
      <Route path="/settings/:section" element={<SettingsPage notify={notify} />} />
      <Route path="/docs" element={<DocsAddress />} />
      <Route path="/files" element={<FilesPage />} />
      <Route path="/files/:repo" element={<FilesPage />} />
      <Route path="/files/:repo/:capsule" element={<FilesPage />} />
      <Route path="/reviews" element={<ReviewsPage />} />
      <Route path="/reviews/:repo/:number" element={<PRReviewPage />} />
      <Route path="/team" element={<TeamPage />} />
      <Route path="/silo/:repo" element={<SiloPoint />} />
      <Route path="/documents" element={<DocumentsPage />} />
      <Route path="/documents/:repo" element={<SpacePage />} />
      <Route path="/documents/local/:id" element={<LocalPageAt />} />
      <Route path="/documents/:repo/*" element={<DocumentAt />} />
      <Route path="/pages" element={<Navigate to="/documents" replace />} />
      <Route path="/pages/open" element={<OpenAddress />} />
      <Route path="/pages/:id" element={<PageAddress />} />
      <Route path="/artifacts" element={<ArtifactsPage />} />
      <Route path="/artifacts/compare" element={<ComparePage />} />
      <Route path="/artifacts/in/*" element={<FoundArtifactPage />} />
      <Route path="/artifacts/:id" element={<ArtifactView />} />
      <Route path="/artifacts/:id/compare" element={<ComparePage />} />
      <Route path="/artifacts/:id/:version" element={<ArtifactView />} />
      <Route path="/conversations" element={<TalksPage />} />
      <Route path="/conversations/:id" element={<TalkPage />} />
      <Route path="/missions/:slug/conversations/:id" element={<TalkPage />} />
      <Route path="/missions/:slug/review/:repo/:name" element={<Review />} />
      <Route path="/missions/:slug/files/:repo/:name" element={<Explorer />} />
      <Route path="/x/:name/*" element={<PluginPage />} />
      <Route path="*" element={<NotFound />} />
    </Routes>
  );
});

// WorkspaceMenu is the workspace's name, opening what's of the workspace
// rather than a place in it: Settings (where the theme is, and the way to
// arrange the sidebar), Keys, and shutting its daemon down, which
// otherwise stays open until it's closed. Places moved out of the sidebar
// are at its top. At its foot is what this is and which: the logotype, and
// the version of ws that serves the page.
function WorkspaceMenu({ places }: { places: Place[] }) {
  // What the menu says of the workspace, each read by itself: it isn't
  // drawn again for anything else that changes.
  const name = useWorkspaceIfRead((w) => w.name) ?? "ws";
  const root = useWorkspaceIfRead((w) => w.root);
  const version = useWorkspaceIfRead((w) => w.version);
  const later = useWorkspaceIfRead((w) => w.later);
  const navigate = useNavigate();
  const { pathname } = useLocation();
  // A place in the menu that wants you says so on the name, as the
  // sidebar's would.
  const counted = places.filter((p) => p.count);
  const shutDown = () => void askShutDown();
  return (
    <Menu
      className="workspace-menu"
      aria-label="The workspace"
      trigger={<WorkspaceName name={name} root={root} wants={counted.length > 0 ? counted.map((p) => `${p.label}: ${p.count} ${p.countTitle ?? "need you"}`).join("\n") : undefined} />}
      // What this is, and which: not an item, so the keys pass it by.
      foot={
        <div className="workspace-menu-foot">
          <Logotype />
          {version && (
            <span className="workspace-menu-version" title={`ws ${version}: the daemon's, which serves this page`}>
              {shortVersion(version)}
            </span>
          )}
        </div>
      }
    >
      {places.length > 0 && (
        <MenuSection aria-label="Places">
          {places.map((p) => (
            <MenuItem key={p.id} id={`place:${p.id}`} textValue={p.label} className={p.match(pathname) ? "menu-current" : undefined} onAction={() => navigate(p.to)}>
              <Icon name={p.icon} />
              <MenuText>{p.label}</MenuText>
              {!!p.count && (
                <Count tone="attention" className="menu-count" title={`${p.count} ${p.countTitle ?? "need you"}`}>
                  {p.count}
                </Count>
              )}
            </MenuItem>
          ))}
        </MenuSection>
      )}
      {places.length > 0 && <MenuSeparator />}
      <MenuItem id="settings" textValue="Settings" onAction={() => navigate("/settings")}>
        <Icon name="sliders" />
        <MenuText>Settings</MenuText>
      </MenuItem>
      <MenuItem id="keys" textValue="Keys" onAction={() => useChromeStore.getState().setHelp(true)}>
        <Icon name="keyboard" />
        <MenuText>Keys</MenuText>
        <Keys k="?" />
      </MenuItem>
      {/* A later release, as the daemon last found: to where it's installed from. */}
      {later && (
        <MenuItem id="later" textValue={`ws ${later} is out`} className="menu-later" onAction={() => navigate("/settings/about#updates")}>
          <Icon name="arrow-up" />
          <MenuText>ws {later} is out</MenuText>
        </MenuItem>
      )}
      <MenuSeparator />
      <MenuItem id="shutdown" textValue="Shut down the daemon" tone="danger" onAction={shutDown}>
        <Icon name="power" />
        <MenuText>Shut down the daemon…</MenuText>
      </MenuItem>
    </Menu>
  );
}

// ConnectionBanner says when the workspace's daemon has gone, and for how long:
// what's on the page may be stale until it's back.
function ConnectionBanner({ connection }: { connection: Connection }) {
  const [lostAt, setLostAt] = useState<number>();
  const [, tick] = useState(0);
  useEffect(() => {
    if (connection === "lost") setLostAt((t) => t ?? Date.now());
    if (connection === "live") setLostAt(undefined);
  }, [connection]);
  useEffect(() => {
    if (!lostAt) return;
    const t = setInterval(() => tick((n) => n + 1), 1000);
    return () => clearInterval(t);
  }, [lostAt]);
  if (!lostAt) return null;
  return (
    <div className="connection-banner" role="status">
      <Spinner />
      <span>
        Lost the workspace's daemon {duration(lostAt)} ago. Reconnecting… What's here may be out of date; <code>ws web</code> starts it again.
      </span>
    </div>
  );
}

// useGlobalKeys: ⌘K finds anything, ⌘P a file, ⌘⇧F text in the files, ?
// shows the keys, and g then m, c, p,
// a, r, t, f or o goes to Missions, Conversations, Pages, Artifacts,
// Reviews, Team, Files or the Orchestrator. They're read before a page's own keys.
function useGlobalKeys() {
  const navigate = useNavigate();
  const openPalette = useChromeStore((c) => c.openFind);
  const openHelp = useCallback(() => useChromeStore.getState().setHelp(true), []);
  useEffect(() => {
    let g = 0;
    const typing = (t: EventTarget | null) => t instanceof HTMLElement && (t.isContentEditable || ["INPUT", "TEXTAREA", "SELECT"].includes(t.tagName));
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && !e.altKey && !e.shiftKey && e.key.toLowerCase() === "k") {
        // In a document being edited, ⌘K with words selected is a link, as
        // in any editor. With nothing selected, it's Find.
        if (e.target instanceof HTMLElement && e.target.closest(".pm") && !window.getSelection()?.isCollapsed) return;
        // With Find open, ⌘K is Find's own: it acts on what the cursor's
        // on there (shell/Palette.tsx).
        if (document.querySelector(".palette-overlay")) return;
        e.preventDefault();
        e.stopPropagation();
        openPalette();
        return;
      }
      if (isCodeSearchKey(e)) {
        // ⌘⇧F is Find, for what the files say: except where a page has a
        // search box of its own (a capsule's files), which is nearer.
        e.preventDefault();
        const does = codeSearchKey(!!document.querySelector("[data-overlay]"));
        if (does.to === "nothing") return; // something's open: the key's its, or nobody's
        e.stopPropagation();
        if (does.to === "own") does.open();
        else openPalette(scopePrefix.code);
        return;
      }
      if ((e.metaKey || e.ctrlKey) && !e.altKey && !e.shiftKey && e.key.toLowerCase() === "p") {
        // ⌘P is Find, for files: except where a page has a file finder of
        // its own (a capsule's files, a review's), which is nearer.
        if (ownsFileFinder(window.location.pathname)) return;
        e.preventDefault();
        if (document.querySelector("[data-overlay]")) return; // something's open: not the browser's print, either
        e.stopPropagation();
        openPalette("/");
        return;
      }
      if (typing(e.target) || e.metaKey || e.ctrlKey || e.altKey || document.querySelector("[data-overlay]")) return;
      if (e.key === "?") {
        e.preventDefault();
        e.stopPropagation();
        openHelp();
        return;
      }
      // g then a key goes somewhere, unless the page uses that key itself
      // (g, first, then o, open it, on a list): the page's keys win.
      const to = { m: "/", c: "/conversations", p: "/documents", d: "/documents", a: "/artifacts", r: "/reviews", t: "/team", f: "/files", o: "/orchestrator" }[e.key];
      if (to && Date.now() - g < leaderWithin && !pageBinds(e.key)) {
        e.preventDefault();
        e.stopPropagation();
        g = 0;
        navigate(to);
        return;
      }
      g = e.key === "g" ? Date.now() : 0;
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [navigate, openPalette, openHelp]);
}

// ownsFileFinder is whether the page at path finds files itself on ⌘P: a
// capsule's or a repo's files.
const ownsFileFinder = (path: string) => /^\/files\/[^/]+/.test(path) || path.includes("/files/");

// useKeyboarding marks the page while you move by keys, so the keyboard's
// cursor shows; moving the mouse hides it again.
function useKeyboarding() {
  useEffect(() => {
    const nav = new Set(["j", "k", "ArrowUp", "ArrowDown", "g", "G", "n", "p"]);
    const onKey = (e: KeyboardEvent) => nav.has(e.key) && !e.metaKey && !e.ctrlKey && document.body.classList.add("keyboarding");
    const onMouse = () => document.body.classList.remove("keyboarding");
    window.addEventListener("keydown", onKey);
    window.addEventListener("mousemove", onMouse);
    return () => {
      window.removeEventListener("keydown", onKey);
      window.removeEventListener("mousemove", onMouse);
    };
  }, []);
}

// useScrollMemory puts a page back where you left it when you come back
// to it (back, forward); somewhere new starts at the top.
const scrolls = new Map<string, number>();
function useScrollMemory() {
  const location = useLocation();
  const how = useNavigationType();
  useLayoutEffect(() => {
    const main = document.getElementById("main");
    if (!main) return;
    const key = location.key;
    const want = how === "POP" ? scrolls.get(key) : undefined;
    // What scrolls is asked for each time: a page may scroll a part of
    // itself (ui/scroller.ts), which isn't there until the page has drawn.
    if (want === undefined) {
      const at = pageScroller();
      if (how !== "POP" && at) at.scrollTop = 0;
    } else {
      // The page may still be loading: try for a moment as it fills.
      let tries = 0;
      const put = () => {
        const at = pageScroller();
        if (at) at.scrollTop = want;
        if ((!at || Math.abs(at.scrollTop - want) > 2) && tries++ < 20) setTimeout(put, 50);
      };
      put();
    }
    // Kept as you scroll: by the time the next page draws, this one's gone.
    // Heard on the way down to whatever scrolls, as a scroll doesn't come
    // back up from a part of the page.
    const keep = (e: Event) => {
      if (e.target === pageScroller()) scrolls.set(key, (e.target as HTMLElement).scrollTop);
    };
    main.addEventListener("scroll", keep, { passive: true, capture: true });
    return () => main.removeEventListener("scroll", keep, { capture: true });
  }, [location.key, how]);
}

// helpKeys are the keys for the page at path.
function helpKeys(path: string): [string, string][] {
  if (path === "/files" || path.startsWith("/files/")) return filesKeys;
  if (path === "/documents") return documentsKeys;
  if (path === "/artifacts") return artifactsKeys;
  if (path === "/artifacts/compare" || /^\/artifacts\/[^/]+\/compare$/.test(path)) return compareKeys;
  if (path.startsWith("/artifacts/in/")) return foundArtifactKeys;
  if (path.startsWith("/artifacts/")) return artifactViewKeys;
  if (path.startsWith("/documents/local/")) return pageKeys;
  if (/^\/documents\/[^/]+\/.+/.test(path)) return new URLSearchParams(window.location.search).has("edit") ? pageKeys : docKeys;
  if (path.startsWith("/documents/")) return spaceKeys;
  if (path === "/reviews") return reviewsKeys;
  if (path === "/team") return teamKeys;
  if (path === "/settings" || path.startsWith("/settings/")) return settingsKeys;
  if (path.startsWith("/reviews/")) return [...prReviewKeys, ...reviewKeys.filter(([k]) => k !== "esc"), ["esc", "Clear the selection, or back to Reviews"]];
  if (path.includes("/review/")) return [...reviewKeys.filter(([k]) => k !== "esc"), ["L", "Land it on the default branch (a repo with no pull requests)"], ...reviewKeys.filter(([k]) => k === "esc")];
  if (path.includes("/files/")) return explorerKeys;
  if (path.endsWith("/artifact")) return artifactKeys;
  if (path.includes("/crew/")) return crewKeys;
  if (path.includes("/conversations/")) return talkKeys;
  if (path === "/conversations") return talksKeys;
  if (path.startsWith("/pages/")) return pageKeys;
  if (path.startsWith("/missions/")) return missionKeys;
  if (path === "/") return listKeys;
  return [];
}

const globalKeys: [string, string][] = [
  ["mod+K", "Find anything: a mission, a file, a pull request, an address"],
  ["mod+P", "Find a file"],
  ["mod+shift+F", "Find text in the files: the repo you're in (⌫ for everywhere), or a capsule's own"],
  ["g m", "Go to Missions"],
  ["g c", "Go to Conversations"],
  ["g d / g p", "Go to Documents"],
  ["g a", "Go to Artifacts"],
  ["g r", "Go to Reviews"],
  ["g t", "Go to Team"],
  ["g f", "Go to Files"],
  ["g o", "Go to the Orchestrator"],
  [".", "Pin what the page is to your Focus, or unpin it"],
  ["\\", "Open or close what's beside the page: what you asked about it"],
  ["?", "These keys"],
  ["esc", "Close, or go back"],
];

// KeyLabel draws a key as the key lists write it: "j / k" and "⏎ or o"
// are alternatives, "g m" a sequence, "mod+K" a chord.
export function KeyLabel({ k }: { k: string }) {
  const alts = k.split(/\s+(?:\/|or)\s+/);
  return (
    <span className="key-label">
      {alts.map((a, i) => (
        <span key={i}>
          {i > 0 && <span className="dim"> or </span>}
          <Keys k={a} />
        </span>
      ))}
    </span>
  );
}

function Help({ keys, onClose }: { keys: [string, string][]; onClose: () => void }) {
  useKeys({ "?": onClose }, { modal: true });
  return (
    <Dialog title="Keys" onClose={onClose} wide>
      <div className="keys-help">
        {keys.length > 0 && (
          <div>
            <h3>On this page</h3>
            <dl>
              {keys.map(([k, what]) => (
                <div key={k}>
                  <dt>
                    <KeyLabel k={k} />
                  </dt>
                  <dd>{what}</dd>
                </div>
              ))}
            </dl>
          </div>
        )}
        <div>
          <h3>Everywhere</h3>
          <dl>
            {globalKeys.map(([k, what]) => (
              <div key={k}>
                <dt>
                  <KeyLabel k={k} />
                </dt>
                <dd>{what}</dd>
              </div>
            ))}
          </dl>
        </div>
      </div>
    </Dialog>
  );
}

// ToBrief is where the brief's own page was: its mission's Brief tab,
// which is where ws still sends you to brief one.
// SaysHere says what the page that's open is of (Find's Here), once, for
// what's beside it (chrome/store.ts): what was opened about a thing is
// there only on that thing's page. A plugin's page is of what its app
// says it shows, which needs what there is to name; any other page's
// address is enough.
const nothingKnown: Known = { repos: new Map(), missions: [], pages: [], talks: [] };

function SaysHere() {
  const workspace = useWorkspace();
  const location = useLocation();
  const known = useRefs()?.known;
  const frame = useFrameHere();
  const setHere = useChromeStore((c) => c.setHere);
  const id = hereAt(location.pathname, location.search, workspace, known ?? nothingKnown, frame)?.id;
  useLayoutEffect(() => setHere(id), [setHere, id]);
  useEffect(() => () => setHere(undefined), [setHere]);
  return null;
}

function ToBrief() {
  const { slug = "" } = useParams();
  return <Navigate to={briefHref(slug)} replace />;
}

function NotFound() {
  useTitle("Not found");
  return (
    <div className="page narrow">
      <Empty icon="search" title="There's nothing here" action={<Link to="/" className={buttonClass()}>Back to missions</Link>}>
        This page doesn't exist, or what it showed has gone.
      </Empty>
    </div>
  );
}

function Notice({ title, children }: { title: string; children: React.ReactNode }) {
  useTitle(title);
  return (
    <div className="notice">
      <div className="mark" aria-hidden>
        w
      </div>
      <h1>{title}</h1>
      <p>{children}</p>
    </div>
  );
}
