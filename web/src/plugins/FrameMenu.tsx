import type { V1 } from "@withworkspace/ui-lib/protocol";
import { refAt, refHref, refName, refOf } from "../refs/refs";
import { useRefs } from "../refs/known";
import { ThingMenu } from "../shell/ThingMenu";
import { Icon, type IconName, icons } from "../ui/Icon";
import { MenuItem, MenuSection, MenuSeparator, MenuText } from "../ui/Menu";

// FrameMenu is ws's menu for a right click in a plugin's app: the app's
// own items first (ws:menu), each said back to the app as it's chosen,
// then what ws does to the ref clicked on, as a row of ws's own pages
// has it (ThingMenu): what can be done to it, your automations' actions
// on it, and opening it in a tab. An icon the app names that isn't one
// of ws's is none.
export function FrameMenu({ items, thing, choose }: { items: V1.MenuItem[]; thing?: string; choose: (id: string) => void }) {
  const refs = useRefs();
  // The ref as ws gave it the app (refKey), or one of ws's names the app
  // wrote itself.
  const page = thing ? (refOf(thing) ?? (refs?.known ? refAt(thing, refs.known) : undefined)) : undefined;
  const to = page ? refHref(page, refs?.known) : undefined;
  const name = page ? refName(page) : undefined;
  const own = items.length > 0 && (
    <MenuSection aria-label="This page's">
      {items.map((it) => (
        <MenuItem key={it.id} id={`page:${it.id}`} textValue={it.label} disabled={it.disabled} tone={it.danger ? "danger" : undefined} onAction={() => choose(it.id)}>
          {it.icon && it.icon in icons ? <Icon name={it.icon as IconName} /> : <span aria-hidden="true" />}
          <MenuText>{it.label}</MenuText>
        </MenuItem>
      ))}
    </MenuSection>
  );
  if (!name) return own || null;
  return (
    <ThingMenu
      to={to}
      thing={name}
      extra={
        own && (
          <>
            {own}
            <MenuSeparator />
          </>
        )
      }
    />
  );
}
