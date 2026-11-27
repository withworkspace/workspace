import { useCallback, useEffect, useLayoutEffect, useMemo, useState } from "react";
import { useLocation, useMatch, useNavigate } from "react-router";
import { NotSignedIn, setSettings } from "./api";
import { useCommands } from "./keymap/useCommands";
import { everywhereKeys } from "./shell/keys";
import { RefsProvider } from "./refs/known";
import { prHref } from "./reviews/Reviews";
import { repoOfPage } from "./shell/find";
import { Palette } from "./shell/Palette/Palette";
import { useFaviconBadge, useFeedNotifications, useNotifications } from "./shell/notify";
import { Arrange } from "./shell/Arrange/Arrange";
import { arrange, type Arrangement, type Arranged, placesOf, sameArrangement, saving } from "./shell/places";
import { FocusKeys } from "./shell/FocusKeys";
import { FocusSection } from "./shell/FocusSection";
import { Pages } from "./shell/Pages";
import { noteVisit, visitAt } from "./shell/recent";
import { wantsRoom } from "./shell/room";
import { SaysHere } from "./shell/SaysHere";
import { useGlobalKeys } from "./shell/useGlobalKeys";
import { useKeyboarding } from "./shell/useKeyboarding";
import { useScrollMemory } from "./shell/useScrollMemory";
import { WhatsNew } from "./shell/WhatsNew/WhatsNew";
import { useThingMenu } from "./shell/ThingMenu";
import { Viewer } from "./ui/Viewer";
import { useMedia } from "./ui/useMedia";
import { setTitleContext } from "./ui/useTitle";
import { toast } from "./ui/toast";
import { Toasts } from "./ui/Toasts";
import { Loading } from "./ui/Loading";
import { humanError } from "./ui/strings";
import { Confirmer } from "./ui/Confirm";
import { ActionInputAsker } from "./automations/ActionInputAsker";
import { ActConfirmer } from "./automations/ActConfirmer";
import { Below } from "./chrome/Below";
import { useBelow } from "./chrome/belowViews";
import { Beside } from "./chrome/Beside";
import { docksQuery, floatBeside, hideSidebar, keepsSidebarQuery, narrowQuery, sidebarPlace, toggleBelow, toggleBeside } from "./chrome/chromeState";
import { ConnectionBanner } from "./chrome/ConnectionBanner";
import { FindButton } from "./chrome/FindButton";
import { Foot } from "./chrome/Foot";
import { FootBelow } from "./chrome/FootBelow";
import { FootBeside } from "./chrome/FootBeside";
import { FootCrewLive } from "./chrome/FootCrewLive";
import { FootReady } from "./chrome/FootReady";
import { FootSends } from "./chrome/FootSends";
import { FootSidebar } from "./chrome/FootSidebar";
import { FootSep } from "./chrome/FootSep";
import { KeysHelp } from "./chrome/KeysHelp";
import { type Need, useArrived, waitingOf } from "./chrome/need";
import { Needs } from "./chrome/Needs";
import { Notice } from "./chrome/Notice";
import { NotifyToggle } from "./chrome/NotifyToggle";
import { NotificationsBell } from "./notifications/NotificationsBell";
import { Places } from "./chrome/Places";
import { Shell } from "./chrome/Shell";
import { Sidebar } from "./chrome/Sidebar";
import { useBeside, useChromeStore, useFrameDim, useFrameExpanded, useFrameHere } from "./chrome/store";
import { newTalkAt } from "./chrome/panes";
import { UpdatedBanner } from "./chrome/UpdatedBanner";
import { usePeekKey } from "./chrome/usePeekKey";
import { WindowError } from "./chrome/WindowError";
import { windowStaleClass } from "./chrome/WindowLost";
import { WorkspaceMenu } from "./chrome/WorkspaceMenu";
import { useFollowWorkspace, usePlugins, useWorkspaceIfRead, useWorkspaceStore } from "./workspace/store";

// App is the window: what's followed of the workspace, the keys
// everywhere, and the chrome round the page (the sidebar, the foot, the
// docks, what's opened over it). The page at the address is Pages'
// (shell/Pages.tsx); what's drawn of the window's own is chrome/Window…
// (stories: Chrome/Window).

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
  // On a window wide enough that the page has the room anyway, it stays
  // open (chromeState's keepsSidebarFrom).
  const wide = useMedia(keepsSidebarQuery);
  const room = (wantsRoom(location.pathname) || (expandedFor !== undefined && expandedFor === onPlugin)) && !arranging && !wide;
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
  // Docked where the window has room for it (chromeState's docksFrom),
  // over the page where it hasn't.
  const roomy = useMedia(docksQuery);
  const setRoomy = useChromeStore((c) => c.setRoomy);
  useLayoutEffect(() => setRoomy(roomy), [setRoomy, roomy]);
  const beside = useBeside();
  // Its toggle's there while there's something to peek at here, or while
  // it's there (docked, it stays with nothing here).
  const besideToggles = beside.all.length > 0 || beside.shown;
  useCommands(everywhereKeys, { beside: () => besideToggles && changeChrome(toggleBeside) });
  // z peeks at what the pointer's on, there (chrome/usePeekKey.ts).
  usePeekKey();
  // C begins a conversation there, about nothing: the place to say the
  // first thing, with the keys. It's a conversation once that's said
  // (panes/NewTalkPane.tsx). Again, it's the same one, with the keys.
  const openBeside = useChromeStore((c) => c.open);
  useCommands(everywhereKeys, { converse: () => openBeside(newTalkAt, { keys: true }) });
  // Below, under the page: what's running (belowViews.tsx), opened from
  // its toggle in the foot, Runs, which says how many run and when one's
  // just started. It has no key yet.
  const { views: belowViews, toggle: belowToggle } = useBelow(!!workspace);
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
  const ready = workspace?.reviews?.ready ?? 0;
  // What's been said on pull requests and is on its way to GitHub, or
  // didn't get there: the pull request that says which is where to go.
  const sends = workspace?.sends;
  const sendsAt = sends?.at?.match(/^(.+)#(\d+)$/);
  const atWork = useMemo(() => (workspace?.missions ?? []).flatMap((m) => m.crew.filter((c) => !c.ended && c.activity === "working").map((c) => ({ m, c }))), [workspace]);
  useEffect(() => setTitleContext(workspace?.name ?? "ws", waiting.length), [workspace?.name, waiting.length]);
  useFaviconBadge(waiting.length > 0);
  const openPath = useCallback((p: string) => navigate(p), [navigate]);
  const notify = useNotifications(waiting, openPath);
  // And what happened, where you've sent its source to the desktop.
  useFeedNotifications(workspace?.notifications?.latest, notify.on, openPath);

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
      toast(`Could not save sidebar layout: ${humanError(String(e))}`, { tone: "danger" });
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
      toast(`Could not reset the sidebar: ${humanError(String(e))}`, { tone: "danger" });
    }
  };
  // The sidebar's toggle, at the foot's start, which says how many need
  // you while the sidebar's out of sight: on a narrow window it opens the
  // drawer, elsewhere it hides and shows the sidebar.
  const out = place === "hidden" || (place === "drawer" && !drawer);
  const toggleSidebar = () => (narrow ? setDrawer((d) => !d) : changeChrome((c) => hideSidebar(c, !c.hidden)));

  if (closed) {
    return (
      <Notice title="Workspace shut down">
        Workspace shutdown. Run <code>ws web</code> in the workspace to start it again.
      </Notice>
    );
  }
  if (error instanceof NotSignedIn) {
    return (
      <Notice title="Not signed in">
        Sign in with the link from your terminal. Run <code>ws web</code> in the workspace.
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
      pageClassName={connection === "lost" && workspace ? windowStaleClass : undefined}
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
              {/* Below's toggle, Runs: what's running, next to the sidebar's. */}
              {workspace && belowViews.length > 0 && <FootBelow open={belowShown} count={belowToggle.count} fresh={belowToggle.fresh} onPress={() => changeChrome(toggleBelow)} />}
              {ready > 0 && <FootSep />}
              <FootReady count={ready} />
              {sends && <FootSep />}
              {sends && <FootSends sending={sends.sending} failed={sends.failed} to={sendsAt ? prHref(sendsAt[1], Number(sendsAt[2])) : undefined} />}
              {atWork.length > 0 && <FootSep />}
              {atWork.map(({ m, c }) => (
                <FootCrewLive key={`${m.slug}:${c.id}`} slug={m.slug} id={c.id} mission={m.title} />
              ))}
            </>
          }
          // The bell (what happened: the feed of notifications, opened over
          // the page), then Beside's toggle, there while there's something to
          // peek at here; marked, closed, with what's new.
          end={
            workspace && (
              <>
                <NotificationsBell />
                {besideToggles && <FootBeside open={beside.shown} waiting={beside.all.length} onPress={() => changeChrome(toggleBeside)} over={beside.over} onOver={roomy ? (over) => changeChrome((c) => floatBeside(c, over)) : undefined} />}
              </>
            )
          }
        />
      }
      overlays={
        <>
          {help && <KeysHelp onClose={() => setHelp(false)} />}
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
          <ActionInputAsker />
          <ActConfirmer />
          <WhatsNew version={workspace?.version} />
          <FocusKeys />
        </>
      }
    >
      <ConnectionBanner connection={connection} />
      <UpdatedBanner version={workspace?.version} />
      {error && <WindowError>{error.message}</WindowError>}
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
