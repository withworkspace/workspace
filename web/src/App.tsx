import { useCallback, useEffect, useLayoutEffect, useMemo, useState } from "react";
import { Button as AriaButton, Header, Menu, MenuItem, MenuSection, MenuTrigger, Popover, Separator } from "react-aria-components";
import { Link, Navigate, NavLink, Route, Routes, useLocation, useMatch, useNavigate, useNavigationType } from "react-router";
import { type Connection, NotSignedIn, type Plugin, shutDownDaemon, usePlugins, useWorkspace, type Workspace } from "./api";
import { ArtifactPage as ArtifactView, ArtifactsPage, artifactKeys as artifactViewKeys, artifactsKeys } from "./artifacts/Artifacts";
import { ComparePage, compareKeys } from "./artifacts/Compare";
import { Explorer, explorerKeys } from "./files/Explorer";
import { docKeys } from "./files/DocPage";
import { FilesPage, filesKeys } from "./files/Files";
import { DocumentsPage, documentsKeys, SpacePage, spaceKeys } from "./documents/Documents";
import { codeSearchKey, isCodeSearchKey, leaderWithin, pageBinds, useKeys } from "./keys";
import { ArtifactPage, artifactKeys } from "./mission/ArtifactPage";
import { BriefView, briefKeys } from "./mission/BriefView";
import { CrewPage, crewKeys } from "./mission/CrewPage";
import { listKeys, MissionList } from "./mission/List";
import { MissionPage, missionKeys } from "./mission/Mission";
import { OrchestratorPage, OrchestratorRunPage } from "./orchestrator/Orchestrator";
import { PluginPage, pluginIcon } from "./plugins/Plugin";
import { RefsProvider } from "./refs/known";
import { SettingsPage } from "./settings/Settings";
import { pageKeys } from "./pages/Pages";
import { DocsAddress, DocumentAt, LocalPageAt, OpenAddress, PageAddress } from "./documents/DocumentAt";
import { Review, reviewKeys } from "./review/Review";
import { PRReviewPage, prReviewKeys } from "./reviews/PRReview";
import { ReviewsPage, reviewsKeys } from "./reviews/Reviews";
import { repoOfPage, scopePrefix } from "./shell/find";
import { Palette } from "./shell/Palette";
import { useFaviconBadge, useNotifications, type Waiting } from "./shell/notify";
import { noteVisit, visitAt } from "./shell/recent";
import { wantsRoom } from "./shell/room";
import { shortVersion } from "./shell/version";
import { TalkPage, TalksPage, talkKeys, talksKeys } from "./talk/Talk";
import { TeamPage, teamKeys } from "./team/Team";
import { Icon, type IconName } from "./ui/Icon";
import { Logotype } from "./ui/Logotype";
import { plain } from "./ui/inline";
import { confirm, Confirmer, Dialog, Empty, humanError, Keys, Loading, setTitleContext, toast, Toasts, useTitle } from "./ui/kit";
import { headlineStatus, Status } from "./ui/status";
import { type ThemeChoice, setTheme, useTheme } from "./ui/theme";
import { duration } from "./ui/time";

export function App() {
  const { workspace, error, connection } = useWorkspace();
  // On a plugin's page, ws is told whose app is on screen.
  const plugins = usePlugins(useMatch("/x/:name/*")?.params.name);
  const [help, setHelp] = useState(false);
  // Find, open: with what it starts with typed ("/" for files, from ⌘P).
  const [palette, setPalette] = useState<string>();
  const [drawer, setDrawer] = useState(false);
  // The daemon closed from the workspace's menu: this page went with it.
  const [closed, setClosed] = useState(false);
  const location = useLocation();
  const navigate = useNavigate();
  // On a page that wants the room the sidebar folds to a rail, which opens
  // over the page on hover.
  const reviewing = wantsRoom(location.pathname);

  // The drawer (on a narrow window) closes on going somewhere.
  useEffect(() => setDrawer(false), [location.pathname]);

  // Where you go is kept, for Find to offer before anything's typed.
  const root = workspace?.root;
  useEffect(() => {
    if (!workspace) return;
    const visit = visitAt(location.pathname, location.search, location.hash, workspace, Date.now());
    if (visit) noteVisit(workspace.root, visit);
    // Only on going somewhere: not each time the workspace changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [location.pathname, location.search, root]);

  useGlobalKeys({ openPalette: (start = "") => setPalette(start), openHelp: () => setHelp(true) });
  useKeyboarding();
  useScrollMemory();

  const waiting = useMemo(() => waitingFor(workspace), [workspace]);
  useEffect(() => setTitleContext(workspace?.name ?? "ws", waiting.length), [workspace?.name, waiting.length]);
  useFaviconBadge(waiting.length > 0);
  const openPath = useCallback((p: string) => navigate(p), [navigate]);
  const notify = useNotifications(waiting, openPath);

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
    <div className={`shell${reviewing ? " reviewing" : ""}${drawer ? " drawer" : ""}`}>
      <div className="topbar">
        <button className="ghost icon-button" onClick={() => setDrawer(true)} aria-label="Open the menu">
          <Icon name="menu" size={18} />
        </button>
        <WorkspaceMenu workspace={workspace} onKeys={() => setHelp(true)} onClosed={() => setClosed(true)} />
        {waiting.length > 0 && <span className="count tone-attention" title="Needs you">{waiting.length}</span>}
      </div>
      <div className="scrim" onClick={() => setDrawer(false)} />
      <aside className="sidebar" aria-label="Workspace">
        <WorkspaceMenu workspace={workspace} onKeys={() => setHelp(true)} onClosed={() => setClosed(true)} />
        <button className="find" onClick={() => setPalette("")}>
          <Icon name="search" size={14} />
          <span>Find…</span>
          <Keys k="mod+K" />
        </button>
        <nav className="nav" aria-label="Places">
          <NavItem to="/" icon="flag" label="Missions" count={waiting.filter((w) => w.kind === "mission").length} match={(p) => p === "/" || p.startsWith("/missions/")} />
          <NavItem
            to="/conversations"
            icon="message"
            label="Conversations"
            count={waiting.filter((w) => w.kind === "talk").length}
            match={(p) => p.startsWith("/conversations")}
          />
          <NavItem to="/documents" icon="file" label="Documents" match={(p) => p === "/documents" || p.startsWith("/documents/") || p.startsWith("/pages/") || p === "/docs"} />
          <NavItem to="/artifacts" icon="artifact" label="Artifacts" match={(p) => p === "/artifacts" || p.startsWith("/artifacts/")} />
          <NavItem
            to="/reviews"
            icon="pr"
            label="Reviews"
            count={workspace?.reviews?.count}
            countTitle="pull requests need you"
            match={(p) => p === "/reviews" || p.startsWith("/reviews/")}
          />
          <NavItem to="/team" icon="user" label="Team" match={(p) => p === "/team"} />
          <NavItem to="/files" icon="layers" label="Files" match={(p) => p === "/files" || p.startsWith("/files/")} />
          <NavItem to="/orchestrator" icon="sparkle" label="Orchestrator" match={(p) => p.startsWith("/orchestrator")} />
          <PluginNav plugins={plugins} />
        </nav>
        {workspace && <Inbox workspace={workspace} path={location.pathname} notify={notify} />}
      </aside>
      <main className={connection === "lost" && workspace ? "stale" : undefined} id="main">
        <ConnectionBanner connection={connection} />
        {error && <div className="banner">{error.message}</div>}
        {!workspace && !error && <Loading what="Opening the workspace…" />}
        {workspace && (
          <RefsProvider workspace={workspace}>
            <Routes>
              <Route path="/" element={<MissionList missions={workspace.missions} />} />
              <Route path="/missions/:slug" element={<MissionPage workspace={workspace} />} />
              <Route path="/missions/:slug/artifact" element={<ArtifactPage workspace={workspace} />} />
              <Route path="/missions/:slug/brief" element={<BriefView workspace={workspace} />} />
              <Route path="/missions/:slug/crew/:id" element={<CrewPage workspace={workspace} />} />
              <Route path="/missions/:slug/orchestrator/:id" element={<OrchestratorRunPage workspace={workspace} />} />
              <Route path="/orchestrator" element={<OrchestratorPage workspace={workspace} />} />
              <Route path="/settings" element={<SettingsPage />} />
              <Route path="/docs" element={<DocsAddress />} />
              <Route path="/files" element={<FilesPage workspace={workspace} />} />
              <Route path="/files/:repo" element={<FilesPage workspace={workspace} />} />
              <Route path="/files/:repo/:capsule" element={<FilesPage workspace={workspace} />} />
              <Route path="/reviews" element={<ReviewsPage />} />
              <Route path="/reviews/:repo/:number" element={<PRReviewPage workspace={workspace} />} />
              <Route path="/team" element={<TeamPage />} />
              <Route path="/documents" element={<DocumentsPage workspace={workspace} />} />
              <Route path="/documents/:repo" element={<SpacePage workspace={workspace} />} />
              <Route path="/documents/local/:id" element={<LocalPageAt workspace={workspace} />} />
              <Route path="/documents/:repo/*" element={<DocumentAt workspace={workspace} />} />
              <Route path="/pages" element={<Navigate to="/documents" replace />} />
              <Route path="/pages/open" element={<OpenAddress />} />
              <Route path="/pages/:id" element={<PageAddress workspace={workspace} />} />
              <Route path="/artifacts" element={<ArtifactsPage workspace={workspace} />} />
              <Route path="/artifacts/compare" element={<ComparePage workspace={workspace} />} />
              <Route path="/artifacts/:id" element={<ArtifactView workspace={workspace} />} />
              <Route path="/artifacts/:id/compare" element={<ComparePage workspace={workspace} />} />
              <Route path="/artifacts/:id/:version" element={<ArtifactView workspace={workspace} />} />
              <Route path="/conversations" element={<TalksPage workspace={workspace} />} />
              <Route path="/conversations/:id" element={<TalkPage workspace={workspace} />} />
              <Route path="/missions/:slug/conversations/:id" element={<TalkPage workspace={workspace} />} />
              <Route path="/missions/:slug/review/:repo/:name" element={<Review workspace={workspace} />} />
              <Route path="/missions/:slug/files/:repo/:name" element={<Explorer workspace={workspace} />} />
              <Route path="/x/:name/*" element={<PluginPage plugins={plugins} ws={workspace?.version} />} />
              <Route path="*" element={<NotFound />} />
            </Routes>
          </RefsProvider>
        )}
      </main>
      {help && <Help keys={helpKeys(location.pathname)} onClose={() => setHelp(false)} />}
      {palette !== undefined && workspace && (
        <Palette
          workspace={workspace}
          waiting={waiting}
          start={palette}
          pageRepo={repoOfPage(location.pathname, workspace.spaces)}
          onClose={() => setPalette(undefined)}
          onHelp={() => {
            setPalette(undefined);
            setHelp(true);
          }}
        />
      )}
      <Toasts />
      <Confirmer />
    </div>
  );
}

const themes: [ThemeChoice, IconName, string][] = [
  ["system", "monitor", "From the system"],
  ["light", "sun", "Light"],
  ["dark", "moon", "Dark"],
];

// WorkspaceMenu is the workspace's name, opening what's of the workspace
// rather than a place in it: Settings, Keys, the theme, and shutting its
// daemon down, which otherwise stays open until it's closed. At its foot
// is what this is and which: the logotype, and the version of ws that
// serves the page.
function WorkspaceMenu({ workspace, onKeys, onClosed }: { workspace?: Workspace; onKeys: () => void; onClosed: () => void }) {
  const name = workspace?.name ?? "ws";
  const navigate = useNavigate();
  const { choice } = useTheme();
  const shutDown = async () => {
    const yes = await confirm({
      title: "Shut down the daemon?",
      body: "Every crew it holds is hung up, as ws daemon stop does, and this page closes with it. ws web starts it again.",
      confirm: "Shut down",
      danger: true,
    });
    if (!yes) return;
    try {
      await shutDownDaemon();
      onClosed();
    } catch (e) {
      toast(`Couldn't shut the daemon down: ${humanError(String(e))}`, { tone: "danger" });
    }
  };
  // data-field: keys typed in the menu are its own, not the page's (keys.ts).
  return (
    <MenuTrigger>
      <AriaButton className="workspace-name" aria-label={`${name}: the workspace's menu`} data-field>
        <span className="mark" aria-hidden>
          {(name.match(/[\p{L}\p{N}]/u)?.[0] ?? "w").toUpperCase()}
        </span>
        <span title={workspace?.root}>{name}</span>
        <Icon name="chevron-down" size={12} className="workspace-name-more" />
      </AriaButton>
      <Popover className="select-popover workspace-menu" placement="bottom start" offset={4} data-field>
        <Menu className="select-list" aria-label="The workspace">
          <MenuItem id="settings" textValue="Settings" className="select-option" onAction={() => navigate("/settings")}>
            <Icon name="sliders" />
            <span className="select-option-text">Settings</span>
          </MenuItem>
          <MenuItem id="keys" textValue="Keys" className="select-option" onAction={onKeys}>
            <Icon name="keyboard" />
            <span className="select-option-text">Keys</span>
            <Keys k="?" />
          </MenuItem>
          <Separator className="menu-separator" />
          <MenuSection
            selectionMode="single"
            disallowEmptySelection
            selectedKeys={[choice]}
            onSelectionChange={(keys) => {
              const [k] = keys === "all" ? [] : [...keys];
              if (k) setTheme(k as ThemeChoice);
            }}
          >
            <Header className="menu-header">Theme</Header>
            {themes.map(([c, icon, label]) => (
              <MenuItem key={c} id={c} textValue={label} className="select-option">
                {({ isSelected }) => (
                  <>
                    <Icon name={icon} />
                    <span className="select-option-text">{label}</span>
                    {isSelected && <Icon name="check" size={14} className="select-check" />}
                  </>
                )}
              </MenuItem>
            ))}
          </MenuSection>
          <Separator className="menu-separator" />
          <MenuItem id="shutdown" textValue="Shut down the daemon" className="select-option danger" onAction={shutDown}>
            <Icon name="power" />
            <span className="select-option-text">Shut down the daemon…</span>
          </MenuItem>
        </Menu>
        {/* What this is, and which: not an item, so the keys pass it by. */}
        <div className="workspace-menu-foot">
          <Logotype />
          {workspace?.version && (
            <span className="workspace-menu-version" title={`ws ${workspace.version}: the daemon's, which serves this page`}>
              {shortVersion(workspace.version)}
            </span>
          )}
        </div>
      </Popover>
    </MenuTrigger>
  );
}

// PluginNav is what plugins add to the sidebar: each view an entry, those
// that name a group (under) together beneath its name.
function PluginNav({ plugins }: { plugins?: Plugin[] }) {
  const entries = (plugins ?? []).flatMap((p) => p.views.map((v) => ({ plugin: p, view: v })));
  if (!entries.length) return null;
  const groups = [...new Set(entries.map((e) => e.view.under ?? ""))].sort((a, b) => (a === "" ? -1 : b === "" ? 1 : a.localeCompare(b)));
  return (
    <>
      {groups.map((group) => (
        <div className="nav-group" key={group}>
          {group && <div className="nav-group-title">{group}</div>}
          {entries
            .filter((e) => (e.view.under ?? "") === group)
            .map(({ plugin, view }) => {
              const to = `/x/${plugin.name}${view.path === "/" ? "" : view.path}`;
              return (
                <NavItem
                  key={to}
                  to={to}
                  icon={plugin.state === "failed" ? "alert" : pluginIcon(view.icon)}
                  label={view.title}
                  count={view.badge}
                  countTitle="to see"
                  match={(p) => p === `/x/${plugin.name}` || p.startsWith(`/x/${plugin.name}/`)}
                />
              );
            })}
        </div>
      ))}
    </>
  );
}

function NavItem({
  to,
  icon,
  label,
  count,
  countTitle = "need you",
  match,
}: {
  to: string;
  icon: IconName;
  label: string;
  count?: number;
  countTitle?: string;
  match: (path: string) => boolean;
}) {
  const { pathname } = useLocation();
  const on = match(pathname);
  return (
    <NavLink to={to} className={on ? "active" : ""} aria-current={on ? "page" : undefined} end>
      <Icon name={icon} size={15} />
      <span>{label}</span>
      {!!count && (
        <span className="count tone-attention" title={`${count} ${countTitle}`}>
          {count}
        </span>
      )}
    </NavLink>
  );
}

// needsYou is whether a mission needs you: its headline's in your court,
// now (the court's "Needs you", as the mission list groups it: stalled ones
// too, as nothing moves on them until you say). One rule, for the list,
// the sidebar, the tab's title, notifications and Find alike.
const needsYou = (m: Workspace["missions"][number]) => m.headline.group === "needs-you";

// talkWaits is whether a workspace conversation is your turn.
const talkWaits = (t: Workspace["conversations"][number]) => !!t.waiting && t.status === "open";

// waitingFor is everything that needs you: missions in your court, and
// conversations where the agent has replied.
function waitingFor(w?: Workspace): (Waiting & { kind: "mission" | "talk" })[] {
  if (!w) return [];
  const missions = w.missions
    .filter(needsYou)
    .map((m) => ({
      kind: "mission" as const,
      key: `m:${m.slug}:${m.headline.kind}:${m.headline.text}`,
      title: m.title,
      body: plain(m.headline.text || m.headline.label),
      path: `/missions/${m.slug}`,
    }));
  const talks = w.conversations
    .filter(talkWaits)
    .map((t) => ({ kind: "talk" as const, key: `t:${t.id}:${t.updated}`, title: t.title || "Conversation", body: "The agent replied: your turn.", path: `/conversations/${t.id}` }));
  return [...missions, ...talks];
}

// Inbox lists what needs you, a click each.
function Inbox({ workspace, path, notify }: { workspace: Workspace; path: string; notify: ReturnType<typeof useNotifications> }) {
  const missions = workspace.missions.filter(needsYou);
  const talks = workspace.conversations.filter(talkWaits);
  if (!missions.length && !talks.length) return null;
  return (
    <section className="inbox" aria-label="Needs you">
      <h2>
        Needs you
        {notify.supported && (
          <button
            className={`ghost icon-button small notify-toggle${notify.on ? " on" : ""}`}
            onClick={notify.toggle}
            title={notify.on ? "Desktop notifications are on: turn them off" : "Notify me on the desktop when something needs me"}
            aria-pressed={notify.on}
          >
            <Icon name="bell" size={12} />
          </button>
        )}
      </h2>
      {missions.slice(0, 8).map((m) => {
        const to = `/missions/${m.slug}`;
        return (
          <Link key={m.slug} to={to} className={path.startsWith(to) ? "active" : ""} title={plain(m.headline.text)}>
            <Status s={headlineStatus(m.headline)} iconOnly />
            <span>{m.title}</span>
          </Link>
        );
      })}
      {missions.length > 8 && (
        <Link to="/" className="inbox-more">
          {missions.length - 8} more {missions.length - 8 === 1 ? "mission" : "missions"}
        </Link>
      )}
      {talks.slice(0, 5).map((t) => {
        const to = `/conversations/${t.id}`;
        return (
          <Link key={t.id} to={to} className={path === to ? "active" : ""}>
            <span className="status tone-attention">
              <Icon name="reply" />
            </span>
            <span>{t.title || t.opening}</span>
          </Link>
        );
      })}
      {talks.length > 5 && (
        <Link to="/conversations" className="inbox-more">
          {talks.length - 5} more {talks.length - 5 === 1 ? "conversation" : "conversations"}
        </Link>
      )}
    </section>
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
      <Icon name="spinner" className="spin" />
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
function useGlobalKeys({ openPalette, openHelp }: { openPalette: (start?: string) => void; openHelp: () => void }) {
  const navigate = useNavigate();
  useEffect(() => {
    let g = 0;
    const typing = (t: EventTarget | null) => t instanceof HTMLElement && (t.isContentEditable || ["INPUT", "TEXTAREA", "SELECT"].includes(t.tagName));
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && !e.altKey && !e.shiftKey && e.key.toLowerCase() === "k") {
        // In a document being edited, ⌘K with words selected is a link, as
        // in any editor. With nothing selected, it's Find.
        if (e.target instanceof HTMLElement && e.target.closest(".pm") && !window.getSelection()?.isCollapsed) return;
        e.preventDefault();
        e.stopPropagation();
        openPalette();
        return;
      }
      if (isCodeSearchKey(e)) {
        // ⌘⇧F is Find, for what the files say: except where a page has a
        // search box of its own (a capsule's files), which is nearer.
        e.preventDefault();
        const does = codeSearchKey(!!document.querySelector(".overlay"));
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
        if (document.querySelector(".overlay")) return; // something's open: not the browser's print, either
        e.stopPropagation();
        openPalette("/");
        return;
      }
      if (typing(e.target) || e.metaKey || e.ctrlKey || e.altKey || document.querySelector(".overlay")) return;
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
    if (want === undefined) {
      if (how !== "POP") main.scrollTop = 0;
    } else {
      // The page may still be loading: try for a moment as it fills.
      let tries = 0;
      const put = () => {
        main.scrollTop = want;
        if (Math.abs(main.scrollTop - want) > 2 && tries++ < 20) setTimeout(put, 50);
      };
      put();
    }
    // Kept as you scroll: by the time the next page draws, this one's gone.
    const keep = () => scrolls.set(key, main.scrollTop);
    main.addEventListener("scroll", keep, { passive: true });
    return () => main.removeEventListener("scroll", keep);
  }, [location.key, how]);
}

// helpKeys are the keys for the page at path.
function helpKeys(path: string): [string, string][] {
  if (path === "/files" || path.startsWith("/files/")) return filesKeys;
  if (path === "/documents") return documentsKeys;
  if (path === "/artifacts") return artifactsKeys;
  if (path === "/artifacts/compare" || /^\/artifacts\/[^/]+\/compare$/.test(path)) return compareKeys;
  if (path.startsWith("/artifacts/")) return artifactViewKeys;
  if (path.startsWith("/documents/local/")) return pageKeys;
  if (/^\/documents\/[^/]+\/.+/.test(path)) return new URLSearchParams(window.location.search).has("edit") ? pageKeys : docKeys;
  if (path.startsWith("/documents/")) return spaceKeys;
  if (path === "/reviews") return reviewsKeys;
  if (path === "/team") return teamKeys;
  if (path.startsWith("/reviews/")) return [...prReviewKeys, ...reviewKeys.filter(([k]) => k !== "esc"), ["esc", "Clear the selection, or back to Reviews"]];
  if (path.includes("/review/")) return [...reviewKeys.filter(([k]) => k !== "esc"), ["L", "Land it on the default branch (a repo with no pull requests)"], ...reviewKeys.filter(([k]) => k === "esc")];
  if (path.includes("/files/")) return explorerKeys;
  if (path.endsWith("/artifact")) return artifactKeys;
  if (/^\/missions\/[^/]+\/brief$/.test(path)) return briefKeys;
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

function NotFound() {
  useTitle("Not found");
  return (
    <div className="page narrow">
      <Empty icon="search" title="There's nothing here" action={<Link to="/" className="button-link">Back to missions</Link>}>
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
