import type { NavigateFunction } from "react-router";
import { type ActionAnswer, type ActionAsk, runPluginAction } from "../api";
import { paneAt } from "../chrome/panes";
import { useChromeStore } from "../chrome/store";
import type { ThingAction } from "../refs/refs";
import type { HereSaid } from "../shell/here";
import { humanError } from "../ui/strings";
import { toast } from "../ui/toast";
import { viewBase } from "./paths";

// A plugin's own action, done from outside Find (its key): run with what
// was open, and what came of it said and gone to, as Find does. One that
// asks something first is asked in Find, which goes on from there.

// An ask Find's to start on: the plugin's action that asked it, what was
// open when it was asked for, and what's been answered so far.
export interface AskInFind {
  plugin: string;
  action: ThingAction;
  here?: HereSaid;
  answers: string[];
  ask: ActionAsk;
}

let pending: AskInFind | undefined;

// takeAskInFind is the ask Find was opened to ask, once: Find takes it as
// it opens (shell/Palette/Palette.tsx).
export function takeAskInFind(): AskInFind | undefined {
  const got = pending;
  pending = undefined;
  return got;
}

// landed is what came of an action: what it said, said; where it opens,
// gone to, as the page or beside it. A plugin has one tab beside: what
// it opens there takes the place of what it had open there, and the
// app's told to go there rather than opened again.
export function landed(res: Pick<ActionAnswer, "say" | "open" | "where">, navigate: NavigateFunction) {
  if (res.say) toast(res.say, { tone: "done" });
  if (!res.open) return;
  if (res.where !== "beside") return navigate(res.open);
  const { chrome, open } = useChromeStore.getState();
  open(res.open, { keys: true, inPlaceOf: pluginTab(chrome.beside.tabs, res.open) });
}

// pluginTab is the tab beside that's the same plugin's app as at, if one
// is: /x/jira/new's is /x/jira/board, say.
export function pluginTab(tabs: readonly string[], at: string): string | undefined {
  const of = paneAt(at);
  if (of?.kind !== "plugin") return undefined;
  return tabs.find((t) => {
    const p = paneAt(t);
    return p?.kind === "plugin" && p.name === of.name;
  });
}

// opensAt is where an action that only opens goes, as ws's address.
export const opensAt = (plugin: string, action: ThingAction) => (action.opens ? `${viewBase(plugin)}${action.opens}` : undefined);

// doPluginAction does one of a plugin's own actions, by its key. One that
// only opens goes there at once; one that's run says so if the plugin's
// slow to answer (it may be starting).
export async function doPluginAction(plugin: string, action: ThingAction, here: HereSaid | undefined, navigate: NavigateFunction) {
  const at = opensAt(plugin, action);
  if (at) return landed({ open: at, where: action.where }, navigate);
  const slow = setTimeout(() => toast(`${action.title}…`), 600);
  try {
    const res = await runPluginAction(plugin, action.id, { answers: [], here });
    if (res.ask) {
      pending = { plugin, action, here, answers: [], ask: res.ask };
      useChromeStore.getState().openFind("");
      return;
    }
    landed(res, navigate);
  } catch (e) {
    toast(humanError((e as Error).message), { tone: "danger" });
  } finally {
    clearTimeout(slow);
  }
}
