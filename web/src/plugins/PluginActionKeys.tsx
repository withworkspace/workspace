import { useEffect, useMemo } from "react";
import { useLocation, useNavigate } from "react-router";
import type { Plugin } from "../api";
import { useFrameHere } from "../chrome/store";
import { type CommandDef, forgetScope, scope } from "../keymap/commands";
import { useCommands } from "../keymap/useCommands";
import { usePref } from "../prefs";
import type { ThingAction } from "../refs/refs";
import { actionHere } from "../shell/find";
import { type Here, hereAt, hereKindOf, hereSaid } from "../shell/here";
import { useSidebarKnown } from "../shell/useSidebarKnown";
import { usePlugins, useWorkspaceIfRead } from "../workspace/store";
import { doPluginAction } from "./doAction";

// PluginActionKeys binds each plugin's own actions to keys, everywhere
// they're for (on: a mission's page, a Jira issue's). Each plugin's are a
// scope of commands (plugin-<name>), which Settings › Keyboard lists with
// the rest: a key you give one is yours, and the keys a plugin suggests
// go after your plugin key, if you've set one (",", then n). An action
// pressed is done as Find does it, told what's open (doAction.ts).
export function PluginActionKeys() {
  const plugins = usePlugins();
  const workspace = useWorkspaceIfRead();
  const known = useSidebarKnown(workspace, plugins);
  const frame = useFrameHere();
  const { pathname, search } = useLocation();
  const [leader] = usePref("pluginKey");
  const here = workspace ? hereAt(pathname, search, workspace, known, frame) : undefined;
  return (
    <>
      {(plugins ?? [])
        .filter((p) => (p.actions ?? []).length > 0)
        .map((p) => (
          <PluginKeys key={p.name} plugin={p} leader={leader} here={here} />
        ))}
    </>
  );
}

// pluginScope is a plugin's own actions as a scope of commands: each its
// title, and the keys the plugin suggests after leader, or none.
export function pluginScope(plugin: Pick<Plugin, "name" | "views">, actions: ThingAction[], leader: string) {
  const defs: Record<string, CommandDef> = {};
  for (const a of actions) defs[a.id] = { title: a.title, keys: leader ? (a.keys ?? []).map((k) => `${leader} ${k}`) : [] };
  return scope(`plugin-${plugin.name}`, plugin.views[0]?.title ?? plugin.name, defs);
}

function PluginKeys({ plugin, leader, here }: { plugin: Plugin; leader: string; here: Here | undefined }) {
  const navigate = useNavigate();
  const actions = useMemo(() => plugin.actions ?? [], [plugin.actions]);
  const commands = useMemo(() => pluginScope(plugin, actions, leader), [plugin.name, plugin.views, actions, leader]);
  const id = `plugin-${plugin.name}`;
  useEffect(() => () => forgetScope(id), [id]);
  const kind = here && hereKindOf(here);
  const handlers: Record<string, (() => void) | undefined> = {};
  for (const a of actions) handlers[a.id] = actionHere(a, kind) ? () => void doPluginAction(plugin.name, a, hereSaid(here, window.getSelection()?.toString()), navigate) : undefined;
  useCommands(commands, handlers);
  return null;
}
