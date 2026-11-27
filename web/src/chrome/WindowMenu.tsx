import type { KeyLike } from "../keymap/keymap";
import { everywhereKeys } from "../shell/keys";
import type { ReactElement } from "react";
import { Count } from "../ui/Count";
import { Icon, type IconName } from "../ui/Icon";
import { Logotype } from "../ui/Logotype";
import { Menu, MenuItem, MenuSection, MenuSeparator, MenuText } from "../ui/Menu";
import { cx } from "../ui/cx";
import css from "./WindowMenu.module.css";

// A place moved out of the sidebar into the workspace's menu: what it's
// called, its icon, its count where it wants you, and whether it's the
// page you're on.
export interface WindowMenuPlace {
  id: string;
  label: string;
  icon: IconName;
  count?: number;
  countTitle?: string;
  keys?: KeyLike; // what goes there (g then m), where a key does
  current?: boolean;
}

// WindowMenu is the workspace's menu, opened from its name (trigger): the
// places moved out of the sidebar at its top, then Settings, Keys, a
// later release of ws where there is one (later), and shutting the
// daemon down. At its foot, not an item, is what this is and which: the
// logotype, and the version of ws that serves the page (version, short
// as it's shown and in full on hover).
export function WindowMenu({
  trigger,
  places,
  version,
  later,
  onPlace,
  onSettings,
  onKeys,
  onLater,
  onShutDown,
}: {
  trigger: ReactElement;
  places: WindowMenuPlace[];
  version?: { full: string; short: string };
  later?: string;
  onPlace: (id: string) => void;
  onSettings: () => void;
  onKeys: () => void;
  onLater: () => void;
  onShutDown: () => void;
}) {
  return (
    <Menu
      className={css.menu}
      aria-label="Workspace"
      trigger={trigger}
      // What this is, and which: not an item, so the keys pass it by.
      foot={
        <div className={css.foot}>
          <Logotype />
          {version && (
            <span className={css.version} title={`Workspace ${version.full}`}>
              {version.short}
            </span>
          )}
        </div>
      }
    >
      {places.length > 0 && (
        <MenuSection aria-label="Places">
          {places.map((p) => (
            <MenuItem key={p.id} id={`place:${p.id}`} textValue={p.label} className={cx(p.current && css.current)} onAction={() => onPlace(p.id)} keys={p.keys}>
              <Icon name={p.icon} />
              <MenuText>{p.label}</MenuText>
              {!!p.count && (
                <Count tone="attention" className={css.count} title={`${p.count} ${p.countTitle ?? "need you"}`}>
                  {p.count}
                </Count>
              )}
            </MenuItem>
          ))}
        </MenuSection>
      )}
      {places.length > 0 && <MenuSeparator />}
      <MenuItem id="settings" textValue="Settings" onAction={onSettings}>
        <Icon name="sliders" />
        <MenuText>Settings</MenuText>
      </MenuItem>
      <MenuItem id="keys" textValue="Keys" onAction={onKeys} keys={everywhereKeys.help}>
        <Icon name="keyboard" />
        <MenuText>Keys</MenuText>
      </MenuItem>
      {/* A later release, as the daemon last found: to where it's installed from. */}
      {later && (
        <MenuItem id="later" textValue={`Workspace ${later} available`} className={css.later} onAction={onLater}>
          <Icon name="arrow-up" />
          <MenuText>Workspace {later} available</MenuText>
        </MenuItem>
      )}
      <MenuSeparator />
      <MenuItem id="shutdown" textValue="Shut down Workspace" tone="danger" onAction={onShutDown}>
        <Icon name="power" />
        <MenuText>Shut down Workspace…</MenuText>
      </MenuItem>
    </Menu>
  );
}
