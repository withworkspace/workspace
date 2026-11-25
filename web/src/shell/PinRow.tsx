import type { FocusPin } from "../api";
import { FocusRow } from "../chrome/FocusRow";
import { type Known, type PageRef, refHref } from "../refs/refs";
import { useWorkspace } from "../workspace/store";
import { useThingMenu } from "./ThingMenu";
import { useStands } from "./useStands";

export type Drag = Pick<React.HTMLAttributes<HTMLAnchorElement>, "onDragStart" | "onDragOver" | "onDragEnd">;

// PinRow is a pin's row: how it stands, as an icon, and what it's called,
// worked out here and drawn by the sidebar's part (chrome/FocusRow.tsx).
export function PinRow({
  pin,
  r,
  known,
  path,
  dragging,
  drop,
  onUnpin,
  drag,
}: {
  pin: FocusPin;
  r: PageRef;
  known?: Known;
  path: string;
  dragging: boolean;
  drop?: "before" | "after";
  onUnpin: () => void;
  drag: Drag;
}) {
  const to = refHref(r, known);
  const workspace = useWorkspace();
  const menu = useThingMenu();
  const { icon, title } = useStands(pin, r, workspace, known);
  const here = r.kind === "mission" ? path === to || path.startsWith(`${to}/`) : path === to;
  return <FocusRow to={to} lead={icon} title={title} says={`${title}${pin.state && r.kind === "thing" ? ` · ${pin.state}` : ""}`} on={here} onUnpin={onUnpin} dragging={dragging} drop={drop} onContextMenu={menu(to, title)} {...drag} />;
}
