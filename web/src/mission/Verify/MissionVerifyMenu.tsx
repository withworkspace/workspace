import type { ReactNode } from "react";
import { Button } from "../../ui/Button";
import { cx } from "../../ui/cx";
import { Icon } from "../../ui/Icon";
import { Menu, MenuItem, MenuSection, MenuSeparator, MenuText } from "../../ui/Menu";
import css from "./MissionVerifyMenu.module.css";

export type MissionVerifyMenuGroup = {
  key: string;
  title: string;
  // Its bundles no longer stand (stale): they sit back.
  back?: boolean;
  bundles: { id: string; title: string; standing: ReactNode }[];
};

// MissionVerifyMenu is the bundles that aren't in the row, behind a
// button: by where they stand, those that no longer stand sitting back.
// end is one at the end of the row.
export function MissionVerifyMenu({ groups, label, onPick, end }: { groups: MissionVerifyMenuGroup[]; label: ReactNode; onPick: (id: string) => void; end?: boolean }) {
  // data-field: keys typed in the menu are its own, not the page's (keys.ts).
  return (
    <Menu
      className={css.menu}
      placement={end ? "bottom end" : "bottom"}
      aria-label="Verification bundles"
      onAction={onPick}
      trigger={
        <Button className={cx(css.more, end && css.end)} data-field>
          {label} <Icon name="chevron-down" size={12} />
        </Button>
      }
    >
      {groups.flatMap((g, i) => [
        ...(i > 0 ? [<MenuSeparator key={`sep-${g.key}`} />] : []),
        <MenuSection key={g.key} title={g.title}>
          {g.bundles.map((b) => (
            <MenuItem key={b.id} id={b.id} textValue={`${b.id} ${b.title}`} className={cx(css.item, g.back && css.back)}>
              <span className="ref">{b.id}</span>
              <MenuText>{b.title}</MenuText>
              {b.standing}
            </MenuItem>
          ))}
        </MenuSection>,
      ])}
    </Menu>
  );
}
