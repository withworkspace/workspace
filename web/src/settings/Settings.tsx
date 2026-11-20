import { Fragment, useEffect, useMemo, useRef, useState } from "react";
import { Link, Navigate, NavLink, useLocation, useParams } from "react-router";
import { getProcess, getRepos, getSettings, type Plugin, setSettings, type Repo, type SettingChange, type SettingsGroup, type SettingsInfo, type Workspace } from "../api";
import { useKeys } from "../keys";
import { useLoaded } from "../load";
import { shortVersion } from "../shell/version";
import { Icon } from "../ui/Icon";
import { Callout, ErrorCard, Keys, Skeleton, toast, useTitle } from "../ui/kit";
import { useTheme } from "../ui/theme";
import { forgetModels, valueSaid } from "./controls";
import { PluginsSection } from "../plugins/Plugins";
import { ProcessAbout, ProcessFile } from "./Process";
import { Updates } from "./Updates";
import { openWhatsNew } from "../shell/WhatsNew";
import { type Page, PageContext, RowOf, SettingCell, usePage, useSetting, without } from "./Row";
import { type Block, find, type Found, keyOf, type Mine, minesOf, name, sectionAt, type Section, sectionsOf, wordsOf } from "./sections";

// The Settings page: your settings for this workspace, in sections, the
// list of them beside the sidebar and the one you're in beside that, to a
// width. Most are the ones in ws.local.toml: each says what it is, and
// where its value comes from when that isn't ws's default (the team's
// ws.toml, or yours). A change is written into ws.local.toml at once, key
// by key, so the rest of the file stays as you wrote it; Reset takes yours
// away, and the team's or the default applies again. A few are kept in
// this browser (the theme, desktop notifications), and say so. Each
// section has an address (/settings/agents), and Find reads them all.

export const settingsKeys: [string, string][] = [["/", "Find a setting"]];

// written is a change as it reads in the file, for its toast.
function written(c: SettingChange): string {
  if (c.clear) return `${name(c)} cleared`;
  const quote = (x: unknown) => (typeof x === "string" ? `"${x}"` : String(x));
  const v = Array.isArray(c.value) ? `[${c.value.map(quote).join(", ")}]` : quote(c.value);
  return `${name(c)} = ${v}`;
}

const fileName = (path: string) => path.split("/").pop() ?? path;

export function SettingsPage({
  workspace,
  plugins,
  notify,
  onArrange,
  onClosed,
}: {
  workspace: Workspace;
  plugins?: Plugin[];
  notify: Page["notify"];
  onArrange: () => void;
  onClosed: () => void;
}) {
  const { section: at } = useParams();
  const anchor = useLocation().hash.slice(1);
  const { value: info, error, set: setInfo, reload: load } = useLoaded<SettingsInfo>(getSettings, [], { keep: true });
  const [query, setQuery] = useState("");
  const field = useRef<HTMLInputElement>(null);
  const sections = useMemo(() => sectionsOf(info), [info]);
  const section = sections.find((s) => s.id === at);
  useTitle(section ? `${section.title} · Settings` : "Settings");
  useKeys({ "/": () => field.current?.focus() });
  // What a harness lists is asked afresh each time you come here.
  useEffect(forgetModels, []);

  // An address with a block named (/settings/missions#orchestrator) shows it.
  const shownAt = section?.id;
  const loaded = !!info;
  useEffect(() => {
    if (anchor && shownAt && loaded) document.getElementById(anchor)?.scrollIntoView({ block: "start" });
  }, [anchor, shownAt, loaded]);

  // change writes changes together; undo is what puts yours back as it was.
  // Writes are made in turn, each answered before the next is sent, so the
  // page is never shown an older answer after a newer one, and no control
  // need wait (or lose its focus) while another's written.
  const writes = useRef(Promise.resolve());
  const file = info ? fileName(info.file) : "ws.local.toml";
  const change = (changes: SettingChange[], undo?: SettingChange[]) => {
    const write = async () => {
      try {
        setInfo(await setSettings(changes));
        toast(`${changes.map(written).join(", ")} in ${file}`, {
          tone: "done",
          action: undo && { label: "Undo", run: () => void change(undo) },
        });
      } catch (e) {
        toast((e as Error).message, { tone: "danger", ms: 8000 });
        await load();
      }
    };
    writes.current = writes.current.then(write);
    return writes.current;
  };

  // /settings is its first section; an address from when the page was one
  // (/settings#orchestrator) is the section that's in now, once the
  // settings say which; and a section there isn't is the first.
  const settled = loaded || !!error;
  if (!at && !anchor) return <Navigate to={`/settings/${sections[0].id}`} replace />;
  if (!at && settled) return <Navigate to={`/settings/${(sectionAt(sections, anchor) ?? sections[0]).id}#${anchor}`} replace />;
  if (at && !section && settled) return <Navigate to={`/settings/${sections[0].id}`} replace />;

  const finding = query.trim() !== "";
  const found = finding ? find(sections, query) : [];
  const all = info?.groups.flatMap((g) => g.settings) ?? [];
  const page: Page = { all, change, marked: finding ? wordsOf(query) : [], notify, arrange: onArrange, closed: onClosed };
  const mine = minesOf(sections).length;
  const unread = info?.unknown?.length ?? 0;

  return (
    <PageContext.Provider value={page}>
      <div className="page settings-page">
        <aside className="settings-side" aria-label="Settings">
          <div className="settings-side-in">
            <div className="settings-side-title">Settings</div>
            <label className="settings-find">
              <Icon name="search" size={13} />
              <input
                ref={field}
                type="text"
                placeholder="Find a setting…"
                value={query}
                spellCheck={false}
                autoComplete="off"
                aria-label="Find a setting"
                onChange={(e) => setQuery(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key !== "Escape") return;
                  setQuery("");
                  e.currentTarget.blur();
                }}
              />
              <Keys k="/" />
            </label>
            <nav className="nav settings-nav" aria-label="Sections">
              {sections.map((sec) => {
                const hits = found.find((f) => f.section.id === sec.id)?.rows.length ?? 0;
                const on = !finding && sec.id === section?.id;
                return (
                  <Fragment key={sec.id}>
                    {sec.heading && <h2>{sec.heading}</h2>}
                    <NavLink to={`/settings/${sec.id}`} className={() => (on ? "active" : finding && hits === 0 ? "none" : "")} aria-current={on ? "page" : undefined} onClick={() => setQuery("")}>
                      <Icon name={sec.icon} size={15} />
                      <span>{sec.title}</span>
                      {finding && hits > 0 && <span className="count">{hits}</span>}
                      {!finding && sec.id === "yours" && unread > 0 && <span className="settings-unread" title={unread === 1 ? "A key ws doesn't read" : `${unread} keys ws doesn't read`} />}
                      {!finding && sec.id === "yours" && mine > 0 && (
                        <span className="count" title={`${mine} set by you`}>
                          {mine}
                        </span>
                      )}
                    </NavLink>
                  </Fragment>
                );
              })}
            </nav>
          </div>
        </aside>
        <div className="settings-body">
          {error && <ErrorCard error={error} retry={load} />}
          {/* An old address, till the settings say which section it's in now. */}
          {!section && !finding && !settled && <Skeleton rows={6} />}
          {finding ?<Results found={found} query={query} clear={() => setQuery("")} /> : section && <SectionView section={section} sections={sections} info={info} loading={!settled} workspace={workspace} plugins={plugins} />}
        </div>
      </div>
    </PageContext.Provider>
  );
}

// Results is what Find found: each section's rows that say it, working
// as they do in their section.
function Results({ found, query, clear }: { found: Found[]; query: string; clear: () => void }) {
  const n = found.reduce((sum, f) => sum + f.rows.length, 0);
  return (
    <>
      <header className="page-head">
        <h1>“{query.trim()}”</h1>
        <span className="dim">
          {n} {n === 1 ? "setting" : "settings"}
        </span>
      </header>
      {found.length === 0 && <div className="quiet-line">No setting says that. What only a file sets (a repo's commands, paths) isn't here.</div>}
      {found.map(({ section, rows }) => (
        <section key={section.id} className="settings-block">
          <h2 className="group">
            <Link to={`/settings/${section.id}`} onClick={clear}>
              {section.title}
            </Link>
          </h2>
          <div className="setting-rows">
            {rows.map((row) => (
              <RowOf key={keyOf(row)} row={row} found />
            ))}
          </div>
        </section>
      ))}
    </>
  );
}

function SectionView({ section, sections, info, loading, workspace, plugins }: { section: Section; sections: Section[]; info?: SettingsInfo; loading: boolean; workspace: Workspace; plugins?: Plugin[] }) {
  // The sections whose rows are all the page's own needn't wait.
  const waits = section.id !== "notifications" && section.id !== "process" && section.id !== "about" && section.id !== "plugins";
  return (
    <>
      <header className="page-head">
        <h1>{section.title}</h1>
      </header>
      <p className="lede">{section.id === "process" ? <ProcessAbout /> : section.about}</p>
      {section.id === "yours" && info && <Yours info={info} sections={sections} />}
      {section.id === "about" ? <About info={info} workspace={workspace} section={section} /> : section.blocks.map((b) => <BlockView key={b.id} block={b} />)}
      {loading && waits && <Skeleton rows={4} />}
      {section.id === "plugins" && <PluginsSection plugins={plugins} />}
      {section.id === "repos" && <Repos branch={workspace.defaultBranch} />}
      {section.id === "process" && <ProcessFile />}
    </>
  );
}

// BlockView is a block: its heading, what it says, and its rows (the
// roles' table, for theirs). Its id is what an address's hash names.
function BlockView({ block }: { block: Block }) {
  if (block.roles) {
    if (block.roles.length === 0) return null;
    return (
      <section id={block.id} className="settings-block">
        <Roles roles={block.roles} />
      </section>
    );
  }
  if (block.rows.length === 0) return null;
  return (
    <section id={block.id} className="settings-block">
      {block.title && <h2 className="group">{block.title}</h2>}
      {block.about && <p className="settings-about">{block.about}</p>}
      <div className="setting-rows">
        {block.rows.map((row) => (
          <RowOf key={keyOf(row)} row={row} />
        ))}
      </div>
    </section>
  );
}

// Roles is every role's agent in one table: the agent CLI it runs in, and
// the model it's given, with why it can't run if it can't, and what it
// follows where it isn't set itself.
function Roles({ roles }: { roles: SettingsGroup[] }) {
  return (
    <div className="settings-roles">
      <div className="settings-roles-head">
        <span>Role</span>
        <span>Agent</span>
        <span>Model</span>
      </div>
      {roles.map((g) => {
        const harness = g.settings.find((s) => s.kind === "harness");
        const model = g.settings.find((s) => s.kind === "model");
        return (
          <div key={g.id} className="settings-role">
            <div className="setting-what">
              <div className="setting-label">
                <span>{g.title}</span>
                <code className="setting-key">[{g.id}]</code>
              </div>
              {g.about && <div className="setting-about">{g.about}</div>}
              {g.runs?.problem && (
                <div className="settings-role-note tone-danger">
                  <Icon name="alert" size={12} /> {g.runs.problem}
                </div>
              )}
              {g.runs?.note && <div className="settings-role-note">{g.runs.note}</div>}
            </div>
            {harness ? <SettingCell s={harness} /> : <span />}
            {model ? <SettingCell s={model} /> : <span />}
          </div>
        );
      })}
    </div>
  );
}

// Repos is the team's repos, as ws.toml has them: shown, not changed.
function Repos({ branch }: { branch?: string }) {
  const { value: repos, error, reload } = useLoaded<Repo[]>(getRepos, [], { keep: true });
  return (
    <section id="repos" className="settings-block">
      <h2 className="group">
        The team's repos
        {repos && <span className="count">{repos.length}</span>}
      </h2>
      <p className="settings-about">
        In <code>ws.toml</code>: shown here, changed there.
      </p>
      {error && <ErrorCard error={error} retry={reload} />}
      {!repos && !error && <Skeleton rows={3} />}
      {repos && (
        <table className="settings-table">
          <thead>
            <tr>
              <th>Repo</th>
              <th>Called</th>
              <th>Aliases</th>
              <th>Group</th>
              <th>Lands by</th>
            </tr>
          </thead>
          <tbody>
            {repos.map((r) => (
              <tr key={r.name}>
                <td className="mono">{r.name}</td>
                <td>
                  <span className="settings-repo">
                    <span className="repo-dot" style={{ background: r.color }} />
                    {r.label}
                  </span>
                </td>
                <td className="mono dim">{r.aliases?.join(", ") || "—"}</td>
                <td className="dim">{r.group || "—"}</td>
                <td className="dim">{r.landing === "direct" ? `straight to ${branch || "its default branch"}` : r.landing ? "pull request" : "—"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  );
}

// Unread is the keys either file has that ws doesn't read.
function Unread({ info }: { info: SettingsInfo }) {
  if (!info.unknown || info.unknown.length === 0) return null;
  return (
    <Callout tone="attention" title="ws doesn't read these">
      Misspelt, or in the wrong file, they're passed over without a word:
      <ul className="settings-unknown">
        {info.unknown.map((u) => (
          <li key={u}>
            <code>{u}</code>
          </li>
        ))}
      </ul>
    </Callout>
  );
}

// Yours is what's yours alone: what your file sets, each with what it
// would be without yours and the way back; the keys ws doesn't read; and
// what this browser keeps.
function Yours({ info, sections }: { info: SettingsInfo; sections: Section[] }) {
  const { notify } = usePage();
  const { choice } = useTheme();
  const mines = minesOf(sections);
  return (
    <>
      <Unread info={info} />
      <section className="settings-block">
        <h2 className="group">
          What {fileName(info.file)} sets <span className="count">{mines.length}</span>
        </h2>
        {mines.length === 0 && <div className="quiet-line">Nothing: every setting is the team's, or ws's default.</div>}
        <div className="settings-mine">
          {mines.map((m) => (
            <MineRow key={name(m.s)} m={m} />
          ))}
        </div>
      </section>
      <section className="settings-block">
        <h2 className="group">In this browser</h2>
        <div className="settings-mine">
          <div className="settings-mine-row">
            <span>
              <Link to="/settings/appearance" className="link">
                Appearance
              </Link>{" "}
              · Theme
            </span>
            <span>{{ system: "As the system has it", light: "Light", dark: "Dark" }[choice]}</span>
          </div>
          <div className="settings-mine-row">
            <span>
              <Link to="/settings/notifications" className="link">
                Notifications
              </Link>{" "}
              · Desktop notifications
            </span>
            <span>{notify.on ? "On" : "Off"}</span>
          </div>
        </div>
      </section>
    </>
  );
}

function MineRow({ m }: { m: Mine }) {
  const { reset } = useSetting(m.s);
  const plain = m.s.kind === "bool" || m.s.kind === "choice";
  return (
    <div className="settings-mine-row">
      <div className="settings-mine-what">
        <span>
          <Link to={`/settings/${m.section.id}`} className="link">
            {m.section.title}
          </Link>{" "}
          · {m.what}
        </span>
        <code className="setting-key">{name(m.s)}</code>
      </div>
      <div className="settings-mine-value">
        <span className={plain ? undefined : "mono"}>{valueSaid(m.s)}</span>
        <span className="dim">otherwise {without(m.s)}</span>
      </div>
      <button className="ghost small" onClick={reset}>
        Reset
      </button>
    </div>
  );
}

// About is which ws this is and what's out that's later, and where the
// workspace keeps what Settings shows. Its blocks are placed among those:
// Updates under the version, the daemon last.
function About({ info, workspace, section }: { info?: SettingsInfo; workspace: Workspace; section: Section }) {
  const process = useLoaded(getProcess, [], { keep: true }).value;
  const updates = section.blocks.find((b) => b.id === "updates");
  return (
    <>
      <section className="settings-block">
        <h2 className="group">ws</h2>
        <dl className="settings-facts">
          <dt>Version</dt>
          <dd>
            <span className="mono" title={workspace.version}>
              {workspace.version ? shortVersion(workspace.version) : "—"}
            </span>
            <div className="dim">The daemon's, which serves this page.</div>
          </dd>
          <dt id="whats-new">What's new</dt>
          <dd>
            <button className="small" onClick={openWhatsNew}>
              Show what's new
            </button>
            <div className="dim">Said once after an upgrade, and kept here.</div>
          </dd>
        </dl>
      </section>
      <section id="updates" className="settings-block">
        <h2 className="group">Updates</h2>
        <Updates running={workspace.version} />
        {updates && (
          <div className="setting-rows">
            {updates.rows.map((row) => (
              <RowOf key={keyOf(row)} row={row} />
            ))}
          </div>
        )}
      </section>
      <section className="settings-block">
        <h2 className="group">This workspace</h2>
        <dl className="settings-facts">
          <dt>Name</dt>
          <dd>{workspace.name}</dd>
          {workspace.org && (
            <>
              <dt>Organisation</dt>
              <dd className="mono">{workspace.org}</dd>
            </>
          )}
          {workspace.defaultBranch && (
            <>
              <dt>Default branch</dt>
              <dd className="mono">{workspace.defaultBranch}</dd>
            </>
          )}
          <dt>Where it is</dt>
          <dd className="mono">{workspace.root}</dd>
          <dt>The team's settings</dt>
          <dd>
            <code title={info?.team}>{info ? fileName(info.team) : "ws.toml"}</code> <span className="dim">in the config repo, changed by commit</span>
          </dd>
          <dt>Yours</dt>
          <dd>
            <code title={info?.file}>{info ? fileName(info.file) : "ws.local.toml"}</code> <span className="dim">beside it, not committed</span>
          </dd>
          {process && (
            <>
              <dt>Process</dt>
              <dd>
                <code title={process.file}>{process.name}</code>{" "}
                <Link to="/settings/process" className="link">
                  as it's written
                </Link>
              </dd>
            </>
          )}
        </dl>
      </section>
      {section.blocks
        .filter((b) => b.id !== "updates")
        .map((b) => (
          <BlockView key={b.id} block={b} />
        ))}
    </>
  );
}
