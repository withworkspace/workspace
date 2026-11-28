import { useEffect, useRef, useState } from "react";
import { keyName, leaderWithin } from "../keys";
import "../keymap/all";
import { allScopes, type Command, type Scope } from "../keymap/commands";
import { holdKeys } from "../keymap/dispatch";
import { type Clash, clashesOf, isOwn, keysOf, resetKeys, setKeys, takeSuggested, useKeymap } from "../keymap/keymap";
import { setPref } from "../prefs";
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

// isPlugin is whether a scope is a plugin's actions (PluginActionKeys).
const isPlugin = (s: Scope) => s.id.startsWith("plugin-");

// listed is names as a list is said: "Review, Documents and Team".
const listed = (names: string[]) => (names.length < 2 ? names.join("") : `${names.slice(0, -1).join(", ")} and ${names[names.length - 1]}`);

// clashSaid is a clash as Settings says it.
const clashSaid = (x: Clash) =>
  x.starts ? `${said(x.key)} starts with ${said(x.starts)}, which is ${x.with.title} (${x.with.scope.title}): pressed twice quickly, it’s ${said(x.key)}` : `${said(x.key)} also does ${x.with.title} (${x.with.scope.title})`;

export function Shortcuts() {
  const map = useKeymap();
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
  // A plugin's action is bound on every page it's for, after the page's
  // own: its keys are weighed against every scope's.
  const also = (c: Command) => (isPlugin(c.scope) ? scopes : []);
  // What a plugin suggests, while it hasn't been taken: with what it'd
  // clash with, taken.
  const suggestion = (c: Command) => {
    const had = keysOf(c, map);
    const left = c.suggested.filter((k) => !had.includes(k));
    if (left.length === 0) return {};
    const would = { ...map, [c.id]: [...had, ...left] };
    const said = lines(
      c,
      clashesOf(c, everywhere, would, also(c)).filter((x) => left.includes(x.key) || (x.starts !== undefined && left.includes(x.starts))),
    );
    return { suggested: left, suggestedClashes: said.clashes, suggestedNotes: said.notes, onUseSuggested: () => takeSuggested(c) };
  };
  // lines is what's said of c's clashes. A plugin's key that a page has
  // too is read after the page's: on that page it's the page's, and that's
  // no trouble, so it's one quiet line of which pages. The rest clash.
  const lines = (c: Command, all: Clash[]) => {
    const plugin = isPlugin(c.scope);
    const pages = (x: Clash) => plugin && !x.starts && !isPlugin(x.with.scope) && x.with.scope !== everywhere;
    const notes: string[] = [];
    const byKey = new Map<string, string[]>();
    for (const x of all.filter(pages)) byKey.set(x.key, [...new Set([...(byKey.get(x.key) ?? []), x.with.scope.title])]);
    for (const [k, on] of byKey) notes.push(`On ${listed(on)}, ${said(k)} is the page's own.`);
    return { clashes: all.filter((x) => !pages(x)).map(clashSaid), notes };
  };
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
          ...lines(c, clashesOf(c, everywhere, map, also(c))),
          ...suggestion(c),
          onAdd: () => add(c),
          onRemove: (k: string) => setKeys(c, keysOf(c).filter((x) => x !== k)),
          onReset: () => resetKeys(c),
          onCancel: stop,
        })),
    }))
    .filter((g) => g.shortcuts.length > 0);

  return <SettingsShortcuts groups={groups} filter={filter} onFilter={setFilter} yours={Object.keys(map).length} onResetAll={() => setPref("keymap", {})} />;
}
