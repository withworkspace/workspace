import { createContext, type ReactNode, useContext } from "react";
import type { Setting, SettingChange } from "../api";
import { askShutDown } from "../shell/daemon";
import type { useNotifications } from "../shell/notify";
import { Icon, type IconName } from "../ui/Icon";
import { Switch } from "../ui/Switch";
import { setTheme, type ThemeChoice, useTheme } from "../ui/theme";
import { Control, valueSaid } from "./controls";
import { name, type Row, words } from "./sections";

// A row of the Settings page: what a setting is and says, then its
// control, and under that where its value comes from, when it isn't ws's
// default. The page's own rows (the theme, notifications, the sidebar,
// the daemon) are drawn the same way.

// What the rows are given by the page: every setting (a change to one can
// take another with it), the way to write, what Find has typed (marked in
// each row), and what the page's own rows do.
export interface Page {
  all: Setting[];
  change: (changes: SettingChange[], undo?: SettingChange[]) => Promise<void>;
  marked: string[];
  notify: Pick<ReturnType<typeof useNotifications>, "supported" | "on" | "toggle">;
  arrange: () => void;
  closed: () => void;
}

export const PageContext = createContext<Page | null>(null);

export function usePage(): Page {
  const page = useContext(PageContext);
  if (!page) throw new Error("a settings row outside the Settings page");
  return page;
}

// Marked is text with what Find has typed marked in it.
export function Marked({ text }: { text: string }) {
  const { marked } = usePage();
  if (marked.length === 0) return <>{text}</>;
  const lower = text.toLowerCase();
  const parts: ReactNode[] = [];
  let at = 0;
  while (at < text.length) {
    // The nearest of the words from here, the longest of those that start there.
    let from = -1;
    let len = 0;
    for (const w of marked) {
      const i = lower.indexOf(w, at);
      if (i >= 0 && (from < 0 || i < from || (i === from && w.length > len))) [from, len] = [i, w.length];
    }
    if (from < 0) break;
    if (from > at) parts.push(text.slice(at, from));
    parts.push(<mark key={from}>{text.slice(from, from + len)}</mark>);
    at = from + len;
  }
  parts.push(text.slice(at));
  return <>{parts}</>;
}

// RowShell is a row's layout: its name, its key in the file, what it
// says; then its control and, under it, the source. wide puts the control
// under the words, for one that wants the room.
export function RowShell({ label, tomlKey, crumb, about, wide, children, source }: { label: string; tomlKey?: string; crumb?: string; about?: ReactNode; wide?: boolean; children: ReactNode; source?: ReactNode }) {
  return (
    <div className={`setting-row${wide ? " wide" : ""}`}>
      <div className="setting-what">
        <div className="setting-label">
          {crumb && <span className="setting-crumb">{crumb} ·</span>}
          <span>
            <Marked text={label} />
          </span>
          {tomlKey && (
            <code className="setting-key">
              <Marked text={tomlKey} />
            </code>
          )}
        </div>
        {about && <div className="setting-about">{about}</div>}
      </div>
      <div className="setting-control">
        {children}
        {source}
      </div>
    </div>
  );
}

const same = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b);
const has = (v: unknown) => v !== undefined && v !== null;

// back is the change that puts yours back as it is now.
const back = (x: Setting): SettingChange => (has(x.yours) ? { section: x.section, key: x.key, value: x.yours } : { section: x.section, key: x.key, clear: true });

// without is what a setting goes back to, reset.
export const without = (s: Setting) =>
  has(s.team) ? `the team's, ${valueSaid(s, s.team) || "none"}` : `the default, ${valueSaid(s, s.default) || (s.kind === "picks" ? "none" : s.kind === "color" ? "ws's own" : s.kind === "places" ? "as ws has them" : "the agent's own")}`;

// useSetting is a setting's two writes: set (a value of yours) and reset
// (yours taken away), each undone from its toast.
export function useSetting(s: Setting): { set: (v: unknown) => Promise<void>; reset: () => void } {
  const { all, change } = usePage();
  const at = { section: s.section, key: s.key };
  const set = (value: unknown) => {
    if (same(value, s.value) && s.source === "yours") return Promise.resolve();
    const changes: SettingChange[] = [{ ...at, value }];
    const undo = [back(s)];
    // A model belongs to the harness it was set for: changing a role's
    // harness takes your model for it away.
    const model = all.find((x) => x.section === s.section && x.kind === "model");
    if (s.kind === "harness" && model && has(model.yours)) {
      changes.push({ section: model.section, key: model.key, clear: true });
      undo.push(back(model));
    }
    return change(changes, undo);
  };
  // The places' two keys are one arrangement: reset together.
  const together = s.kind === "places" ? all.filter((x) => x.kind === "places" && has(x.yours)) : [s];
  const reset = () => void change(together.map((x) => ({ section: x.section, key: x.key, clear: true })), together.map(back));
  return { set, reset };
}

// Source says where a setting's value comes from, when it isn't ws's
// default: the team's file, or yours, with what it's over and the way
// back. brief says only whose (a table's cell: the rest is its title).
export function Source({ s, brief }: { s: Setting; brief?: boolean }) {
  const { reset } = useSetting(s);
  if (s.source === "default") return null;
  if (s.source === "team") {
    return (
      <div className="setting-source" title="From the team's ws.toml: changed there">
        <span className="setting-dot team" />
        the team's
      </div>
    );
  }
  const over = has(s.team) ? `the team's is ${valueSaid(s, s.team)}` : has(s.default) ? `default ${valueSaid(s, s.default)}` : "";
  return (
    <div className="setting-source" title={`From your ws.local.toml${brief && over ? `: ${over}` : ""}`}>
      <span className="setting-dot yours" />
      yours{!brief && over ? ` · ${over}` : ""}
      <button className="link" onClick={reset} title={`Back to ${without(s)}`}>
        Reset
      </button>
    </div>
  );
}

export function SettingRow({ s, crumb }: { s: Setting; crumb?: string }) {
  const { set, reset } = useSetting(s);
  return (
    <RowShell label={s.label} tomlKey={name(s)} crumb={crumb} about={s.about && <Marked text={s.about} />} wide={s.kind === "picks"} source={<Source s={s} />}>
      <Control s={s} set={set} reset={reset} />
    </RowShell>
  );
}

// SettingCell is a setting as a table's cell: its control, and whose it is.
export function SettingCell({ s }: { s: Setting }) {
  const { set, reset } = useSetting(s);
  return (
    <div className="setting-control">
      <Control s={s} set={set} reset={reset} />
      <Source s={s} brief />
    </div>
  );
}

const themes: [ThemeChoice, IconName, string][] = [
  ["system", "monitor", "System"],
  ["light", "sun", "Light"],
  ["dark", "moon", "Dark"],
];

// What this browser keeps says so, where a file's setting says whose it is.
const here = (
  <div className="setting-source" title="Kept in this browser, not in a file">
    <span className="setting-dot team" />
    this browser
  </div>
);

// ThemeRow is the theme: the workspace's menu has it too.
function ThemeRow() {
  const { choice } = useTheme();
  return (
    <RowShell label={words.theme.label} about={<Marked text={words.theme.about} />} source={here}>
      <div className="segmented" role="group" aria-label="Theme">
        {themes.map(([c, icon, label]) => (
          <button key={c} type="button" aria-pressed={choice === c} onClick={() => setTheme(c)}>
            <Icon name={icon} size={13} />
            {label}
          </button>
        ))}
      </div>
    </RowShell>
  );
}

// PlacesRow is the sidebar's arrangement: arranged in the sidebar itself,
// which the button turns to. Here, whose it is, and the way back.
function PlacesRow({ settings }: { settings: Setting[] }) {
  const { arrange } = usePage();
  const mine = settings.find((s) => s.source === "yours") ?? settings.find((s) => s.source === "team");
  return (
    <RowShell label={words.places.label} tomlKey={settings.map((s) => (s === settings[0] ? name(s) : s.key)).join(" · ")} about={<Marked text={words.places.about} />} source={mine && <Source s={mine} brief />}>
      <button type="button" onClick={arrange}>
        <Icon name="grip" size={13} />
        Arrange the sidebar…
      </button>
    </RowShell>
  );
}

// NotifyRow is desktop notifications, on or off: the bell beside Needs
// you in the sidebar is the same switch.
function NotifyRow() {
  const { notify } = usePage();
  const blocked = notify.supported && !notify.on && Notification.permission === "denied";
  return (
    <RowShell
      label={words.notify.label}
      about={
        <>
          <Marked text={words.notify.about} />
          {!notify.supported && " This browser has none."}
          {blocked && " The browser has them blocked for this page: allow them in its own settings first."}
        </>
      }
      source={here}
    >
      <Switch on={notify.on} disabled={!notify.supported} onChange={notify.toggle} aria-label={words.notify.label} />
    </RowShell>
  );
}

// DaemonRow shuts the daemon down, asking first: the workspace's menu
// has it too.
function DaemonRow() {
  const { closed } = usePage();
  return (
    <RowShell label={words.daemon.label} about={<Marked text={words.daemon.about} />}>
      <button type="button" className="danger" onClick={() => void askShutDown(closed)}>
        <Icon name="power" size={13} />
        Shut down…
      </button>
    </RowShell>
  );
}

// RowOf draws a row, whichever it is. found says it's out of its block
// (Find's results), where a setting says whose it is.
export function RowOf({ row, found }: { row: Row; found?: boolean }) {
  switch (row.kind) {
    case "setting":
      return <SettingRow s={row.s} crumb={found ? row.crumb : undefined} />;
    case "places":
      return <PlacesRow settings={row.settings} />;
    case "theme":
      return <ThemeRow />;
    case "notify":
      return <NotifyRow />;
    case "daemon":
      return <DaemonRow />;
  }
}
