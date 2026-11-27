import { useEffect, useRef, useState } from "react";
import { keyName, leaderWithin } from "../keys";
import "../keymap/all";
import { allScopes, type Command, type Scope } from "../keymap/commands";
import { holdKeys } from "../keymap/dispatch";
import { clashesOf, isOwn, keysOf, resetKeys, setKeys, useKeymap } from "../keymap/keymap";
import { setPref, usePref } from "../prefs";
import { everywhereKeys } from "../shell/keys";
import { keysSaid } from "../ui/Keys";
import { SettingsShortcuts, type SettingsShortcutsGroup } from "./SettingsShortcuts";

// Shortcuts is Settings › Keyboard's list of every command a key does,
// with yours: what it reads (the scopes, the keymap you've set) and
// does (sets a key by having you press it), for SettingsShortcuts to
// draw.
//
// Setting a key: + holds every key (keymap/dispatch.ts, holdKeys), so
// none does what it does while it's pressed, and what's pressed next is
// the key. A second soon after (leaderWithin) makes it one then another
// (g then m). esc gives up.
const said = (k: string) => keysSaid(k).join(" ");

export function Shortcuts() {
  const map = useKeymap();
  // Plugins' scopes are made again with your plugin key (PluginActionKeys):
  // drawn again with them.
  usePref("pluginKey");
  const [filter, setFilter] = useState("");
  const [setting, setSetting] = useState<{ c: Command; keys: string[] }>();
  const release = useRef<() => void>(undefined);
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);

  const stop = () => {
    release.current?.();
    release.current = undefined;
    clearTimeout(timer.current);
    setSetting(undefined);
  };
  useEffect(() => stop, []);

  const add = (c: Command) => {
    stop();
    const pressed: string[] = [];
    const done = () => {
      const k = pressed.join(" ");
      const had = keysOf(c);
      if (k && !had.includes(k)) setKeys(c, [...had, k]);
      stop();
    };
    release.current = holdKeys((e) => {
      const k = keyName(e);
      if (k === "Escape") return stop();
      pressed.push(k);
      clearTimeout(timer.current);
      if (pressed.length === 2) return done();
      setSetting({ c, keys: [...pressed] });
      timer.current = setTimeout(done, leaderWithin);
    });
    setSetting({ c, keys: [] });
  };

  const everywhere = everywhereKeys.find.scope;
  const scopes = [everywhere, ...allScopes().filter((s) => s !== everywhere)];
  const q = filter.trim().toLowerCase();
  const matches = (c: Command) => !q || c.title.toLowerCase().includes(q) || c.scope.title.toLowerCase().includes(q) || keysOf(c, map).some((k) => k.toLowerCase().includes(q) || keysSaid(k).join(" ").toLowerCase().includes(q));
  const groups: SettingsShortcutsGroup[] = scopes
    .map((s: Scope) => ({
      title: s.title,
      shortcuts: s.commands
        .filter((c) => !c.fixed && matches(c))
        .map((c) => ({
          id: c.id,
          title: c.title,
          keys: keysOf(c, map),
          yours: !isOwn(c, map),
          setting: setting?.c === c ? setting.keys : undefined,
          clashes: clashesOf(c, everywhere, map).map((x) => (x.starts ? `${said(x.key)} starts with ${said(x.starts)}, which is ${x.with.title} (${x.with.scope.title}): pressed twice quickly, it’s ${said(x.key)}` : `${said(x.key)} also does ${x.with.title} (${x.with.scope.title})`)),
          onAdd: () => add(c),
          onRemove: (k: string) => setKeys(c, keysOf(c).filter((x) => x !== k)),
          onReset: () => resetKeys(c),
          onCancel: stop,
        })),
    }))
    .filter((g) => g.shortcuts.length > 0);

  return <SettingsShortcuts groups={groups} filter={filter} onFilter={setFilter} yours={Object.keys(map).length} onResetAll={() => setPref("keymap", {})} />;
}
