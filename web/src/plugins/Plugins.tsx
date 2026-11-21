import { useEffect, useRef, useState } from "react";
import { Link } from "react-router";
import { addPlugin, ApiError, checkUpdates, getInstalledPlugins, type InstalledPlugin, type Plugin, type PluginsInstalled, removePlugin, updatePlugin, type WantedPlugin } from "../api";
import { type Reload, useLoaded } from "../load";
import { Icon } from "../ui/Icon";
import { Callout, Copyable, Dialog, Empty, ErrorCard, humanError, Skeleton, toast } from "../ui/kit";
import { Time } from "../ui/time";
import { addsOf, changedSaid, fromOf, outCount, repoLink, stateOf, updateOf, wantedFrom } from "./installed";
import { pluginIcon } from "./Plugin";
import { Chip, chipClass } from "../ui/Chip";
import { Button } from "../ui/Button";
import { GroupHead } from "../ui/GroupHead";

// Plugins, a section of Settings: the workspace's plugins, each with where it's from,
// where it stands and what it adds; adding one, from the repo it's
// published at or a folder on this machine; updating one to its repo's
// latest release; and removing one. It's what ws plugin add, update and
// remove do. The daemon looks once a day for later releases, and what it
// found is said on each row; Check for updates looks now.

// Failed is what didn't work, to say above the list: a sentence, and what
// a build printed on the way, where it got that far.
interface Failed {
  what: string;
  error: string;
  output?: string;
}

const failed = (what: string, e: unknown): Failed => ({
  what,
  error: humanError((e as Error).message),
  output: e instanceof ApiError ? e.output : undefined,
});

export function PluginsSection({ plugins: live }: { plugins?: Plugin[] }) {
  const { value: info, error, reload } = useLoaded<PluginsInstalled>(getInstalledPlugins, [], { keep: true });
  // What the sidebar follows says when a plugin starts, stops, comes or
  // goes: the list is read again then.
  const stamp = (live ?? []).map((p) => `${p.name}:${p.state}:${p.starts}`).join(" ");
  const first = useRef(true);
  useEffect(() => {
    if (first.current) first.current = false;
    else void reload();
  }, [stamp, reload]);

  const [problem, setProblem] = useState<Failed>();
  const [looking, setLooking] = useState(false);
  const look = () => {
    setLooking(true);
    checkUpdates()
      .then(async (up) => {
        await reload();
        toast(up.plugins > 0 ? `${up.plugins} ${up.plugins === 1 ? "plugin has" : "plugins have"} a later release` : "No plugin has a later release", { tone: "done" });
      })
      .catch((e: Error) => toast(`Couldn't look: ${humanError(e.message)}`, { tone: "danger", ms: 8000 }))
      .finally(() => setLooking(false));
  };
  const installed = info?.plugins ?? [];
  const out = outCount(installed);
  const fromRepo = installed.some((p) => p.from.repo);

  return (
    <div className="plugins-section">
      <AddPlugin reload={reload} onFailed={setProblem} />
      {info && fromRepo && (
        <div className="plugins-updates">
          {out > 0 && (
            <Chip tone="attention">
              {out} {out === 1 ? "update" : "updates"}
            </Chip>
          )}
            <span className="dim plugins-looked">
              {info.checked ? (
                <>
                  Looked for updates <Time iso={info.checked} as="relative" />
                </>
              ) : (
                "Not looked for updates yet"
              )}
            </span>
            <Button size="small" onClick={look} disabled={looking} title="Ask each plugin's repo for its latest release now">
              <Icon name={looking ? "spinner" : "rotate"} size={12} /> {looking ? "Looking…" : "Check for updates"}
            </Button>
        </div>
      )}
      {problem && (
        <Callout tone="danger" title={problem.what} actions={<Button onClick={() => setProblem(undefined)}>Dismiss</Button>}>
          {problem.error}
          {problem.output && <pre className="plugins-output">{problem.output}</pre>}
        </Callout>
      )}
      {error && !info && <ErrorCard error={error} retry={reload} />}
      {!info && !error && <Skeleton rows={4} />}
      {info && (
        <>
          <section>
            <GroupHead count={installed.length}>
              Installed
            </GroupHead>
            {installed.length === 0 ? (
              <Empty icon="box" title="No plugins yet">
                Add one above: <code>withworkspace/plugin-plugin-dev</code> writes a plugin of your own, and runs it here as you change it.
              </Empty>
            ) : (
              <ul className="rows plugin-rows">
                {installed.map((p) => (
                  <PluginRow key={p.name} p={p} icon={live?.find((l) => l.name === p.name)?.views[0]?.icon} reload={reload} onFailed={setProblem} />
                ))}
              </ul>
            )}
          </section>
          {info.wanted.length > 0 && (
            <section>
              <GroupHead count={info.wanted.length}>
                The team's, not installed here
              </GroupHead>
              <p className="quiet-line">
                In <code>ws.toml</code>, under <code>[plugins]</code>.
              </p>
              <ul className="rows plugin-rows">
                {info.wanted.map((w) => (
                  <WantedRow key={w.name} w={w} reload={reload} onFailed={setProblem} />
                ))}
              </ul>
            </section>
          )}
          {info.checks === false && fromRepo && (
            <p className="quiet-line">
              The daemon isn't looking for updates by itself: <Link to="/settings/about#updates">Look for updates</Link> is off.
            </p>
          )}
        </>
      )}
    </div>
  );
}

// AddPlugin is the field a plugin's added from: the repo it's published
// at, or a folder on this machine.
function AddPlugin({ reload, onFailed }: { reload: Reload; onFailed: (f?: Failed) => void }) {
  const [from, setFrom] = useState("");
  const [busy, setBusy] = useState(false);
  const typed = from.trim();
  const add = () => {
    if (!typed || busy) return;
    setBusy(true);
    onFailed(undefined);
    addPlugin(typed)
      .then(async (c) => {
        setFrom("");
        await reload();
        toast(changedSaid(c, "added"), { tone: "done", ms: 6000 });
      })
      .catch(async (e: Error) => {
        // One that's installed and didn't start is in the list, failed.
        await reload();
        onFailed(failed(`Couldn't add ${typed}`, e));
      })
      .finally(() => setBusy(false));
  };
  return (
    <form
      className="plugins-add"
      onSubmit={(e) => {
        e.preventDefault();
        add();
      }}
    >
      <label className="plugins-add-field">
        <span className="plugins-add-label">Add a plugin</span>
        <input
          type="text"
          value={from}
          placeholder="owner/repo, or a folder on this machine"
          spellCheck={false}
          autoComplete="off"
          autoCapitalize="off"
          disabled={busy}
          onChange={(e) => setFrom(e.target.value)}
          aria-describedby="plugins-add-about"
        />
      </label>
      <Button variant="primary" type="submit" disabled={!typed || busy}>
        <Icon name={busy ? "spinner" : "plus"} size={12} /> {busy ? "Adding…" : "Add"}
      </Button>
      <p id="plugins-add-about" className="plugins-add-about">
        A repo on GitHub is installed from its latest release (<code>owner/repo@v1.2.0</code> for another), checked against the release's checksums; one with no build for
        this machine is built here from its source. A folder is copied in, by its whole path or from <code>~</code>. A plugin is a program that runs as you: add one as
        you'd install any other.
      </p>
    </form>
  );
}

// PluginRow is an installed plugin. icon is its first view's, as the
// sidebar draws it.
function PluginRow({ p, icon, reload, onFailed }: { p: InstalledPlugin; icon?: string; reload: Reload; onFailed: (f?: Failed) => void }) {
  const [busy, setBusy] = useState<"update" | "remove">();
  const [removing, setRemoving] = useState(false);
  const state = stateOf(p);
  const update = updateOf(p);
  const adds = addsOf(p);
  // By its name, not its view's title: a view may be called anything
  // (plugin-dev's is "Plugins"), and what it adds is said under it.
  const title = p.name;
  const doUpdate = () => {
    setBusy("update");
    onFailed(undefined);
    updatePlugin(p.name)
      .then(async (c) => {
        await reload();
        toast(changedSaid(c, "updated"), { tone: "done", ms: 6000 });
      })
      .catch(async (e: Error) => {
        await reload();
        onFailed(failed(`Couldn't update ${p.name}`, e));
      })
      .finally(() => setBusy(undefined));
  };
  const doRemove = async (data: boolean) => {
    setRemoving(false);
    setBusy("remove");
    onFailed(undefined);
    try {
      await removePlugin(p.name, data);
      await reload();
      toast(`Removed ${p.name}${data ? ", and what it kept" : ""}`, { tone: "done" });
    } catch (e) {
      await reload();
      onFailed(failed(`Couldn't remove ${p.name}`, e));
    } finally {
      setBusy(undefined);
    }
  };
  return (
    <li className={`row plugin-row state-${p.state}`}>
      <span className="plugin-row-icon" aria-hidden>
        <Icon name={p.state === "failed" ? "alert" : pluginIcon(icon)} size={16} />
      </span>
      <div className="row-main plugin-row-main">
        <div className="plugin-row-top">
          {p.path && p.state !== "failed" ? (
            <Link to={p.path} className="plugin-row-title" title={`Open ${title}`}>
              {title}
            </Link>
          ) : (
            <span className="plugin-row-title">{title}</span>
          )}
          {p.version && <span className="mono dim">{p.version}</span>}
          <Chip tone={state.tone} title={state.title}>
            {state.label}
          </Chip>
          {update.out && (
            <a className={chipClass("attention", "plugin-row-out")} href={repoLink(p.from.repo ?? "", p.latest)} target="_blank" rel="noopener noreferrer" title="What's in it, on GitHub">
              {p.latest} is out
              <Icon name="arrow-up-right" size={10} />
            </a>
          )}
        </div>
        {p.description && <div className="plugin-row-about">{p.description}</div>}
        <div className="row-sub plugin-row-sub">
          <span>
            {p.from.repo ? (
              <>
                From{" "}
                <a href={repoLink(p.from.repo)} target="_blank" rel="noopener noreferrer">
                  {fromOf(p)}
                </a>
              </>
            ) : (
              fromOf(p)
            )}
            {p.from.at && (
              <>
                , fetched <Time iso={p.from.at} as="relative" />
              </>
            )}
          </span>
          {adds.length > 0 && <span>Adds {adds.join(" · ")}</span>}
        </div>
        {p.state === "setup" && (
          <div className="plugin-row-note tone-attention">
            It can't be used until {p.missing?.length === 1 ? "this is" : "these are"} set: <span className="mono">{(p.missing ?? []).join(", ")}</span>. In a terminal:{" "}
            <Copyable text={`ws plugin config ${p.name} --ask`} className="mono">
              ws plugin config {p.name} --ask
            </Copyable>
          </div>
        )}
        {p.state === "failed" && (
          <div className="plugin-row-note tone-danger">
            {humanError(p.error ?? "It didn't start")} Its log:{" "}
            <Copyable text={`ws plugin logs ${p.name}`} className="mono">
              ws plugin logs {p.name}
            </Copyable>
          </div>
        )}
        {p.state !== "failed" && p.error && <div className="plugin-row-note">It last logged: {p.error}</div>}
        {p.refused?.map((r) => (
          <div key={r} className="plugin-row-note">
            Left out: {r}
          </div>
        ))}
        {p.updateError && <div className="plugin-row-note">Couldn't look for a later release: {p.updateError}</div>}
      </div>
      <span className="plugin-row-actions">
        {update.can && (
          <Button variant={update.out ? "primary" : "ghost"} size="small" onClick={doUpdate} disabled={!!busy} title={update.title}>
            <Icon name={busy === "update" ? "spinner" : update.out ? "arrow-up" : "rotate"} size={11} /> {busy === "update" ? "Updating…" : update.label}
          </Button>
        )}
        <Button variant="ghost" size="small" onClick={() => setRemoving(true)} disabled={!!busy} title="Stop it, and take it out of the workspace">
          <Icon name={busy === "remove" ? "spinner" : "x"} size={11} /> {busy === "remove" ? "Removing…" : "Remove"}
        </Button>
      </span>
      {removing && <RemoveDialog p={p} onClose={() => setRemoving(false)} onRemove={doRemove} />}
    </li>
  );
}

// RemoveDialog asks before a plugin's removed, and whether what it kept
// for itself goes with it.
function RemoveDialog({ p, onClose, onRemove }: { p: InstalledPlugin; onClose: () => void; onRemove: (data: boolean) => void }) {
  const [data, setData] = useState(false);
  return (
    <Dialog title={`Remove ${p.name}?`} onClose={onClose}>
      <p className="dialog-text">
        It's stopped and taken out of the workspace: its view leaves the sidebar, its commands go
        {p.names?.length ? `, and its things' names (${p.names.join(", ")}) are words again wherever they're written` : ""}. Its config in <code>ws.toml</code> and{" "}
        <code>ws.local.toml</code> is yours to remove.
      </p>
      <label className="plugins-remove-data">
        <input type="checkbox" checked={data} onChange={(e) => setData(e.target.checked)} />
        <span>
          Remove what it kept for itself too: its data, and its secrets
          <span className="dim"> Left alone, adding it again picks up where it was.</span>
        </span>
      </label>
      <div className="dialog-actions">
        <Button variant="ghost" onClick={onClose}>
          Cancel
        </Button>
        <Button variant="danger" onClick={() => onRemove(data)} autoFocus>
          {data ? "Remove, and what it kept" : "Remove"}
        </Button>
      </div>
    </Dialog>
  );
}

// WantedRow is one of the team's plugins that isn't installed here, with
// the way to add it.
function WantedRow({ w, reload, onFailed }: { w: WantedPlugin; reload: Reload; onFailed: (f?: Failed) => void }) {
  const [busy, setBusy] = useState(false);
  const add = () => {
    setBusy(true);
    onFailed(undefined);
    addPlugin(wantedFrom(w))
      .then(async (c) => {
        await reload();
        toast(changedSaid(c, "added"), { tone: "done", ms: 6000 });
      })
      .catch(async (e: Error) => {
        await reload();
        onFailed(failed(`Couldn't add ${w.name}`, e));
      })
      .finally(() => setBusy(false));
  };
  return (
    <li className="row plugin-row wanted">
      <span className="plugin-row-icon" aria-hidden>
        <Icon name="circle-dashed" size={16} />
      </span>
      <div className="row-main plugin-row-main">
        <div className="plugin-row-top">
          <span className="plugin-row-title">{w.name}</span>
          {w.version && <span className="mono dim">{w.version}</span>}
        </div>
        <div className="row-sub plugin-row-sub">
          <span className="mono">{w.source}</span>
        </div>
      </div>
      <span className="plugin-row-actions">
        <Button size="small" onClick={add} disabled={busy || !w.source} title={`Install it from ${w.source}`}>
          <Icon name={busy ? "spinner" : "plus"} size={11} /> {busy ? "Adding…" : "Add"}
        </Button>
      </span>
    </li>
  );
}
