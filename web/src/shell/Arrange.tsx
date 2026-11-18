import { useState } from "react";
import { Button as AriaButton, DropIndicator, GridList, GridListItem, isTextDropItem, useDragAndDrop, type DropItem, type ItemDropTarget } from "react-aria-components";
import { Icon } from "../ui/Icon";
import { headed, move, type Arranged, type Place } from "./places";

// Arrange is the sidebar while you arrange it: its places in their order,
// and under them those in the workspace's menu, each dragged by its grip
// (or picked up with Enter on it, from the keys) to another place in
// either list. Each also has a button that moves it across. Nothing is
// written until Done; Cancel leaves it as it was, Reset as the team's (or
// as it comes).

const kind = "application/x-ws-place";

export function Arrange({
  start,
  canReset,
  onDone,
  onCancel,
  onReset,
}: {
  start: Arranged;
  canReset: boolean;
  onDone: (arranged: Arranged) => void;
  onCancel: () => void;
  onReset: () => void;
}) {
  const [arranged, setArranged] = useState(start);
  const put = (ids: string[], into: keyof Arranged, at?: { id: string; after: boolean }) => setArranged((a) => move(a, ids, into, at));
  return (
    <div className="arrange" data-field>
      <div className="arrange-head">Arrange the sidebar</div>
      <PlaceList list="sidebar" places={arranged.sidebar} put={put} label="In the sidebar" empty="Nothing in the sidebar: drag a place here." />
      <div className="arrange-head arrange-menu-head">
        <Icon name="chevron-down" size={12} />
        In the workspace's menu
      </div>
      <PlaceList list="menu" places={arranged.menu} put={put} label="In the workspace's menu" empty="Drag a place here to keep it in the menu, out of the sidebar." />
      <div className="arrange-actions">
        {canReset && (
          <button className="ghost small" onClick={onReset} title="Take your arrangement away: the team's, or as ws has it">
            Reset
          </button>
        )}
        <span className="arrange-spacer" />
        <button className="ghost small" onClick={onCancel}>
          Cancel
        </button>
        <button className="primary small" onClick={() => onDone(arranged)}>
          Done
        </button>
      </div>
    </div>
  );
}

function PlaceList({
  list,
  places,
  put,
  label,
  empty,
}: {
  list: keyof Arranged;
  places: Place[];
  put: (ids: string[], into: keyof Arranged, at?: { id: string; after: boolean }) => void;
  label: string;
  empty: string;
}) {
  const other: keyof Arranged = list === "sidebar" ? "menu" : "sidebar";
  const titles = headed(places);
  const at = (t: ItemDropTarget) => ({ id: String(t.key), after: t.dropPosition === "after" });
  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) => [...keys].map((k) => ({ [kind]: String(k), "text/plain": places.find((p) => p.id === k)?.label ?? String(k) })),
    acceptedDragTypes: [kind],
    getDropOperation: () => "move",
    onReorder: (e) => put([...e.keys].map(String), list, at(e.target)),
    onInsert: async (e) => put(await idsOf(e.items), list, at(e.target)),
    onRootDrop: async (e) => put(await idsOf(e.items), list),
    renderDropIndicator: (target) => <DropIndicator target={target} className="arrange-drop" />,
  });
  return (
    <GridList
      aria-label={label}
      className="arrange-list"
      data-list={list}
      items={places}
      dragAndDropHooks={dragAndDropHooks}
      renderEmptyState={() => <div className="arrange-empty">{empty}</div>}
    >
      {(p) => {
        const title = titles[places.indexOf(p)];
        return (
          <GridListItem id={p.id} textValue={p.label} className="arrange-item">
            <AriaButton slot="drag" className="arrange-grip" aria-label={`Move ${p.label}`}>
              <Icon name="grip" size={14} />
            </AriaButton>
            <Icon name={p.icon} size={15} />
            <span className="arrange-label">
              {p.label}
              {title && <span className="arrange-under">{title}</span>}
            </span>
            <AriaButton
              className="arrange-across"
              onPress={() => put([p.id], other)}
              aria-label={list === "sidebar" ? `Move ${p.label} into the menu` : `Move ${p.label} back to the sidebar`}
            >
              <Icon name={list === "sidebar" ? "arrow-down" : "arrow-up"} size={13} />
            </AriaButton>
          </GridListItem>
        );
      }}
    </GridList>
  );
}

const idsOf = (items: DropItem[]) => Promise.all(items.filter(isTextDropItem).map((i) => i.getText(kind)));
