import { useEffect, useMemo } from "react";
import { useLocation, useNavigate } from "react-router";
import type { Plugin } from "../api";
import { useFrameHere } from "../chrome/store";
import { type CommandDef, forgetScope, scope } from "../keymap/commands";
import { useCommands } from "../keymap/useCommands";
import type { ThingAction } from "../refs/refs";
import { actionHere } from "../shell/find";
import { type Here, hereAt, hereKindOf, hereSaid } from "../shell/here";
import { useSidebarKnown } from "../shell/useSidebarKnown";
import { usePlugins, useWorkspaceIfRead } from "../workspace/store";
import { doPluginAction } from "./doAction";

// PluginActionKeys binds each plugin's own actions to keys, everywhere
// they're for (on: a mission's page, a Jira issue's). Each plugin's are a
// scope of commands (plugin-<name>), which Settings › Keyboard lists with
// the rest. None has a key until you give it one: what a plugin suggests
// is said there, yours to use or not (two plugins may suggest the same).
// They're read after the page's own, so a page's key stays the page's.
// An action pressed is done as Find does it, told what's open
// (doAction.ts).
export function PluginActionKeys() {
  const plugins = usePlugins();
  const workspace = useWorkspaceIfRead();
  const known = useSidebarKnown(workspace, plugins);
  const frame = useFrameHere();
  const { pathname, search } = useLocation();
  const here = workspace ? hereAt(pathname, search, workspace, known, frame) : undefined;
  return (
    <>
      {(plugins ?? [])
        .filter((p) => (p.actions ?? []).length > 0)
        .map((p) => (
          <PluginKeys key={p.name} plugin={p} here={here} />
        ))}
    </>
  );
}

// pluginScope is a plugin's own actions as a scope of commands: each its
// title, no keys of its own, and the keys the plugin suggests.
export function pluginScope(plugin: Pick<Plugin, "name" | "views">, actions: ThingAction[]) {
  const defs: Record<string, CommandDef> = {};
  for (const a of actions) defs[a.id] = { title: a.title, keys: [], suggested: a.keys ?? [] };
  return scope(`plugin-${plugin.name}`, plugin.views[0]?.title ?? plugin.name, defs);
}

function PluginKeys({ plugin, here }: { plugin: Plugin; here: Here | undefined }) {
  const navigate = useNavigate();
  const actions = useMemo(() => plugin.actions ?? [], [plugin.actions]);
  const commands = useMemo(() => pluginScope(plugin, actions), [plugin.name, plugin.views, actions]);
  const id = `plugin-${plugin.name}`;
  useEffect(() => () => forgetScope(id), [id]);
  const kind = here && hereKindOf(here);
  const handlers: Record<string, (() => void) | undefined> = {};
  for (const a of actions) handlers[a.id] = actionHere(a, kind) ? () => void doPluginAction(plugin.name, a, hereSaid(here, window.getSelection()?.toString()), navigate) : undefined;
  useCommands(commands, handlers, { late: true });
  return null;
}
