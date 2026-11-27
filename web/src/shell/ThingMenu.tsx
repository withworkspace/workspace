import { useMemo } from "react";
import { useNavigate } from "react-router";
import type { Workspace } from "../api";
import { isOutside } from "../refs/refs";
import { useContextMenu } from "../ui/ContextMenu";
import { Icon } from "../ui/Icon";
import { MenuItem, MenuSection, MenuSeparator, MenuText } from "../ui/Menu";
import { useWorkspaceIfRead } from "../workspace/store";
import { actsFor, subjectOf } from "./acts";
import type { Item } from "./find";
import { doToThing, isThingDoing } from "./perform";
import { visitAt } from "./recent";
import { doAction, useActions } from "../automations/actions";
import type { AutomationAction } from "../api";

// The menu of a row that goes somewhere, opened on it with the other
// button: what can be done to the thing it goes to, which is what Find
// lists on that thing (acts.ts), and the way to open it in a tab of its
// own, which the browser's menu would have had.

// actsAt is what can be done to the thing a link goes to: nothing, for
// somewhere that isn't one of ws's own things, or one with nothing to do
// to it.
export function actsAt(to: string, w: Workspace | undefined): Item[] {
  if (!w || isOutside(to)) return [];
  const at = new URL(to, "http://ws");
  const subject = subjectOf(visitAt(at.pathname, at.search, at.hash, w, 0)?.id);
  return subject ? actsFor(subject, w) : [];
}

// refAt is the ref of the thing a link goes to, as ws names it (a
// mission's slug, w3, demo#12, RAM-881): undefined for somewhere that
// isn't one.
export function refAt(to: string, w: Workspace | undefined): string | undefined {
  if (!w || isOutside(to)) return undefined;
  const at = new URL(to, "http://ws");
  const id = visitAt(at.pathname, at.search, at.hash, w, 0)?.id;
  return id ? refOfId(id) : undefined;
}

// refOfId is the ref a row of Find's id names, for the kinds of thing an
// automation's action is on.
export function refOfId(id: string): string | undefined {
  const i = id.indexOf(":");
  const kind = id.slice(0, i);
  const rest = id.slice(i + 1);
  if (!rest) return undefined;
  switch (kind) {
    case "mission":
    case "talk":
    case "artifact":
    case "pr":
      return rest;
    case "thing":
      return rest.slice(rest.indexOf("/") + 1) || undefined;
  }
  return undefined;
}

// ThingMenu is what such a menu holds, as the workspace stands while it's
// open: what can be done to the thing (by where the row goes, to), your
// automations' actions on it (as the server says, once it's said), and
// opening it in a tab. thing is the thing by its ref, where there's no
// row's link to go by (a right click in a plugin's frame); extra is more,
// before the rest: what the frame's app added.
export function ThingMenu({ to, thing, extra }: { to?: string; thing?: string; extra?: React.ReactNode }) {
  const workspace = useWorkspaceIfRead();
  const navigate = useNavigate();
  const acts = to ? actsAt(to, workspace) : [];
  const on = thing ?? (to ? refAt(to, workspace) : undefined);
  const automations = useActions(on);
  const perform = (it: Item) => {
    if (it.doing) {
      if (isThingDoing(it.doing)) doToThing(it.doing);
    } else if (it.to && isOutside(it.to)) window.open(it.to, "_blank", "noopener");
    else if (it.to) navigate(it.to);
  };
  return (
    <>
      {extra}
      {acts.length > 0 && (
        <MenuSection aria-label="Actions">
          {acts.map((it) => (
            <MenuItem key={it.id} id={it.id} textValue={it.title} onAction={() => perform(it)}>
              <Icon name={it.icon ?? "play"} />
              <MenuText>{it.title}</MenuText>
            </MenuItem>
          ))}
        </MenuSection>
      )}
      {acts.length > 0 && automations.length > 0 && <MenuSeparator />}
      {automations.length > 0 && (
        <MenuSection aria-label="Your automations">
          {automations.map((a: AutomationAction) => (
            <MenuItem key={`${a.automation}/${a.title}`} id={`automation:${a.automation}/${a.title}`} textValue={a.title} onAction={() => void doAction(a, on, navigate)}>
              <Icon name="bolt" />
              <MenuText>{a.input?.length ? `${a.title}…` : a.title}</MenuText>
            </MenuItem>
          ))}
        </MenuSection>
      )}
      {to && (acts.length > 0 || automations.length > 0) && <MenuSeparator />}
      {to && (
        <MenuItem id="new-tab" textValue="Open in new tab" onAction={() => window.open(to, "_blank", "noopener")}>
          <Icon name="external" />
          <MenuText>Open in new tab</MenuText>
        </MenuItem>
      )}
    </>
  );
}

// useThingMenu gives a row its menu: called with where the row goes (and
// what it's called, for a screen reader), it's the row's onContextMenu.
export function useThingMenu(): (to: string | undefined, title?: string) => React.MouseEventHandler<Element> | undefined {
  const menu = useContextMenu();
  return useMemo(() => (to, title) => (to ? menu(title ?? "This row", () => <ThingMenu to={to} />) : undefined), [menu]);
}
