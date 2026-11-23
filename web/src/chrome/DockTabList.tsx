import type { ReactNode } from "react";
import { Button as AriaButton, Header, Menu, MenuItem, MenuSection, MenuTrigger, Popover } from "react-aria-components";
import { Icon } from "../ui/Icon";
import css from "./DockTabList.module.css";

// One of the tabs, as the list has it: what its tab shows, in full.
export type ListedTab = {
  id: string;
  name: string;
  lead?: ReactNode;
  // It's the one in front.
  on: boolean;
  onPick: () => void;
};

type DockTabListProps = {
  // The tabs, in the lots the row has them in (Beside: what's here
  // because of the page, then yours). A lot with a name is headed by it.
  lots: { name?: string; tabs: ListedTab[] }[];
};

// DockTabList is every tab a dock has, as a list under a button at the
// end of its row: there when they don't all fit, so one that's out of
// sight is a press and a pick away, with its name in full. For whoever's
// on a mouse, where scrolling a row sideways is the slow way.
export function DockTabList({ lots }: DockTabListProps) {
  const shown = lots.filter((l) => l.tabs.length > 0);
  const count = shown.reduce((n, l) => n + l.tabs.length, 0);
  return (
    <MenuTrigger>
      <AriaButton data-ui="" data-field aria-label={`All ${count} tabs`} className={css.button}>
        <span className={css.count}>{count}</span>
        <Icon name="chevron-down" size={11} />
      </AriaButton>
      {/* data-field: keys typed in the list are its own, not the page's (keys.ts). */}
      <Popover className={`select-popover ${css.pop}`} placement="bottom end" offset={4} data-field>
        <Menu className="select-list" aria-label="Every tab">
          {shown.map((lot, i) => (
            <MenuSection key={lot.name ?? i} aria-label={lot.name ?? "Tabs"}>
              {lot.name && shown.length > 1 && <Header className="menu-header">{lot.name}</Header>}
              {lot.tabs.map((t) => (
                <MenuItem key={t.id} id={t.id} textValue={t.name} className={`select-option ${css.option}${t.on ? ` ${css.on}` : ""}`} onAction={t.onPick}>
                  <span className={css.lead}>{t.lead}</span>
                  <span className="select-option-text">{t.name}</span>
                  {t.on && <Icon name="check" size={12} className={css.check} />}
                </MenuItem>
              ))}
            </MenuSection>
          ))}
        </Menu>
      </Popover>
    </MenuTrigger>
  );
}
