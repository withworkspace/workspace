import { DropIndicator, GridList, GridListItem, isTextDropItem, useDragAndDrop, type DropItem, type ItemDropTarget } from "react-aria-components";
import { Button } from "../../ui/Button";
import { Icon } from "../../ui/Icon";
import type { Arranged, Place } from "../places";
import css from "./ArrangeList.module.css";

// Dragging has no part of the library's yet, so this is react-aria's own
// (ui/aria.test.ts lets this file import it).

const kind = "application/x-ws-place";

type Where = { id: string; after: boolean };

type ArrangeListProps = {
  // Which list it is: the sidebar's, or the workspace's menu's.
  list: keyof Arranged;
  places: Place[];
  // What each is under, where that's said (headed, places.ts).
  under: (string | undefined)[];
  // Places dropped in it, from it or the other: where they go, ahead of or
  // after a place, or at the end.
  onPut: (ids: string[], at?: Where) => void;
  // A place moved across, to the other list, by its button.
  onAcross: (id: string) => void;
};

const words = {
  sidebar: {
    label: "In the sidebar",
    empty: "Nothing in the sidebar: drag a place here.",
    across: (label: string) => `Move ${label} into the menu`,
    icon: "arrow-down",
  },
  menu: {
    label: "In workspace menu",
    empty: "Drag a place here to move it out of the sidebar.",
    across: (label: string) => `Move ${label} back to the sidebar`,
    icon: "arrow-up",
  },
} as const;

// ArrangeList is one list of places while the sidebar's arranged: a row
// each, dragged by its grip (or picked up with Enter, from the keys), a
// line where a dragged one would go, and a button that moves it across.
export function ArrangeList({ list, places, under, onPut, onAcross }: ArrangeListProps) {
  const say = words[list];
  const at = (t: ItemDropTarget) => ({ id: String(t.key), after: t.dropPosition === "after" });
  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) => [...keys].map((k) => ({ [kind]: String(k), "text/plain": places.find((p) => p.id === k)?.label ?? String(k) })),
    acceptedDragTypes: [kind],
    getDropOperation: () => "move",
    onReorder: (e) => onPut([...e.keys].map(String), at(e.target)),
    onInsert: async (e) => onPut(await idsOf(e.items), at(e.target)),
    onRootDrop: async (e) => onPut(await idsOf(e.items)),
    renderDropIndicator: (target) => <DropIndicator target={target} className={css.drop} />,
  });
  return (
    <GridList
      aria-label={say.label}
      className={css.list}
      data-list={list}
      items={places}
      dragAndDropHooks={dragAndDropHooks}
      renderEmptyState={() => <div className={css.empty}>{say.empty}</div>}
    >
      {(p) => {
        const title = under[places.indexOf(p)];
        return (
          <GridListItem id={p.id} textValue={p.label} className={css.item}>
            <Button variant="bare" slot="drag" className={css.grip} aria-label={`Move ${p.label}`}>
              <Icon name="grip" size={14} />
            </Button>
            <Icon name={p.icon} size={15} className={css.placeIcon} />
            <span className={css.label}>
              {p.label}
              {title && <span className={css.under}>{title}</span>}
            </span>
            <Button variant="bare" className={css.across} onClick={() => onAcross(p.id)} aria-label={say.across(p.label)}>
              <Icon name={say.icon} size={13} />
            </Button>
          </GridListItem>
        );
      }}
    </GridList>
  );
}

const idsOf = (items: DropItem[]) => Promise.all(items.filter(isTextDropItem).map((i) => i.getText(kind)));
