import { type KeyLike, useSaid } from "../keymap/keymap";
import type { ReactElement, ReactNode } from "react";
import { Header, Menu as AriaMenu, MenuItem as AriaMenuItem, MenuSection as AriaMenuSection, MenuTrigger, Separator } from "react-aria-components";
import { cx } from "./cx";
import { keyHint, pointed } from "./keyHint";
import { Keys } from "./Keys";
import css from "./Menu.module.css";
import { type Placement, PopoverSurface } from "./Popover";

// Menu is a button and the list of things to do it opens: places to go,
// ways to do something, one of several to pick. Its trigger is a Button;
// its children are MenuItems, in MenuSections with MenuSeparators between
// where they're in groups. react-aria's menu does the rest: the arrows,
// typing a name, Enter, Escape, and what a screen reader hears.
//
// onAction is called with the id of the item chosen, unless the item has
// an onAction of its own. A foot is under the list, not an item: the keys
// pass it by. Keys typed in the menu are its own unless it says pageKeys.
export function Menu({
  trigger,
  "aria-label": label,
  placement,
  onAction,
  pageKeys,
  className,
  foot,
  children,
}: {
  trigger: ReactElement;
  "aria-label": string;
  placement?: Placement;
  onAction?: (id: string) => void;
  pageKeys?: boolean;
  className?: string;
  foot?: ReactNode;
  children: ReactNode; // MenuItems, MenuSections, MenuSeparators
}) {
  return (
    <MenuTrigger>
      {trigger}
      <PopoverSurface placement={placement} pageKeys={pageKeys} className={className}>
        <MenuList aria-label={label} onAction={onAction}>
          {children}
        </MenuList>
        {foot}
      </PopoverSurface>
    </MenuTrigger>
  );
}

// MenuList is a menu's list by itself, for a part that opens one some
// other way than from a button (ContextMenu, at the pointer): it's told
// when to close, since no trigger's around it to say, and takes the focus
// as it opens.
export function MenuList({
  "aria-label": label,
  onAction,
  onClose,
  focused,
  className,
  children,
}: {
  "aria-label": string;
  onAction?: (id: string) => void;
  onClose?: () => void;
  focused?: boolean;
  className?: string;
  children: ReactNode; // MenuItems, MenuSections, MenuSeparators
}) {
  return (
    <AriaMenu className={cx(css.list, className)} aria-label={label} autoFocus={focused} onClose={onClose} onAction={onAction && ((key) => onAction(String(key)))}>
      {children}
    </AriaMenu>
  );
}

// MenuItem is one thing to do: its id, the words typing finds it by
// (textValue), and what it shows, which is a row: an icon, MenuText, a
// count or a key at its end. A danger one is in red. keys is the key
// that does the same without the menu: it's drawn at the row's end, and
// hinted at when the item's picked with the pointer (ui/keyHint.ts). A
// disabled one is shown, and can't be chosen.
export function MenuItem({
  id,
  textValue,
  onAction,
  tone,
  keys: given,
  disabled,
  className,
  children,
}: {
  id: string;
  textValue: string;
  onAction?: () => void;
  tone?: "danger";
  keys?: KeyLike;
  disabled?: boolean;
  className?: string;
  children: ReactNode;
}) {
  const keys = useSaid(given);
  return (
    <AriaMenuItem id={id} textValue={textValue} isDisabled={disabled} onAction={onAction} onPress={keys ? (e) => pointed(e) && keyHint(keys) : undefined} className={cx(css.item, tone === "danger" && css.danger, className)}>
      {children}
      {keys && <Keys k={keys} />}
    </AriaMenuItem>
  );
}

// MenuText is an item's words: they take the room there is, and are cut
// short with an ellipsis where there isn't enough.
export function MenuText({ children }: { children: ReactNode }) {
  return <span className={css.text}>{children}</span>;
}

// MenuSection is a group of items, with a title over it, or, with none,
// an aria-label that says what the group is.
export function MenuSection({ title, "aria-label": label, children }: { title?: string; "aria-label"?: string; children: ReactNode }) {
  return (
    <AriaMenuSection className={css.section} aria-label={label}>
      {title && <Header className={css.header}>{title}</Header>}
      {children}
    </AriaMenuSection>
  );
}

// MenuSeparator is a rule between groups.
export function MenuSeparator() {
  return <Separator className={css.separator} />;
}
