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

// ThingMenu is what such a menu holds, as the workspace stands while it's
// open.
export function ThingMenu({ to }: { to: string }) {
  const workspace = useWorkspaceIfRead();
  const navigate = useNavigate();
  const acts = actsAt(to, workspace);
  const perform = (it: Item) => {
    if (it.doing) {
      if (isThingDoing(it.doing)) doToThing(it.doing);
    } else if (it.to && isOutside(it.to)) window.open(it.to, "_blank", "noopener");
    else if (it.to) navigate(it.to);
  };
  return (
    <>
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
      {acts.length > 0 && <MenuSeparator />}
      <MenuItem id="new-tab" textValue="Open in new tab" onAction={() => window.open(to, "_blank", "noopener")}>
        <Icon name="external" />
        <MenuText>Open in new tab</MenuText>
      </MenuItem>
    </>
  );
}

// useThingMenu gives a row its menu: called with where the row goes (and
// what it's called, for a screen reader), it's the row's onContextMenu.
export function useThingMenu(): (to: string | undefined, title?: string) => React.MouseEventHandler<Element> | undefined {
  const menu = useContextMenu();
  return useMemo(() => (to, title) => (to ? menu(title ?? "This row", () => <ThingMenu to={to} />) : undefined), [menu]);
}
