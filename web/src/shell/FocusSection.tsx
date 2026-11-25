import { type DragEvent, useEffect, useRef, useState } from "react";
import { refNamed } from "../refs/refs";
import { Focus } from "../chrome/Focus";
import { usePlugins, useWorkspace } from "../workspace/store";
import { pageRefAt, pinNameOf } from "./pinNames";
import { PinRow } from "./PinRow";
import { useFocus } from "./useFocus";
import { useSidebarKnown } from "./useSidebarKnown";

// Focus: what you've pinned, of anything ws names, kept in the sidebar
// below its places (ops/focus.go). Each is drawn as it stands, and what
// ends (a mission landed, a conversation resolved) leaves it by itself.
// With nothing pinned there's nothing: not even the heading. You pin from
// a card, a page's Pin (or . on it), ⌘. in Find, or by dragging a link
// to it onto the sidebar; unpin from its row, or the same again.

// FocusSection is the sidebar's Focus: your pins, a row each, nothing at
// all when there are none. A row's dragged to put them in order; a link
// to something pinnable, dragged from the page, is pinned where it's
// dropped (and while one's dragged, Focus is there to drop it on).
export function FocusSection({ path }: { path: string }) {
  const workspace = useWorkspace();
  const plugins = usePlugins();
  const focus = useFocus();
  const known = useSidebarKnown(workspace, plugins);
  // A row of Focus being moved, or a link from the page being brought in:
  // its pin's name either way.
  const [moving, setMoving] = useState<string>();
  const [incoming, setIncoming] = useState<string>();
  const [over, setOver] = useState<{ name: string; after: boolean }>();
  const knownNow = useRef(known);
  knownNow.current = known;
  useEffect(() => {
    const start = (e: globalThis.DragEvent) => {
      const a = (e.target as Element | null)?.closest?.("a[href]") as HTMLAnchorElement | null;
      if (!a || a.closest("[data-focus]")) return;
      const url = new URL(a.href, window.location.href);
      if (url.origin !== window.location.origin) return;
      const r = pageRefAt(url.pathname, url.search, knownNow.current);
      const name = r && pinNameOf(r);
      if (name) setIncoming(name);
    };
    const end = () => {
      setIncoming(undefined);
      setOver(undefined);
    };
    window.addEventListener("dragstart", start);
    window.addEventListener("dragend", end);
    window.addEventListener("drop", end);
    return () => {
      window.removeEventListener("dragstart", start);
      window.removeEventListener("dragend", end);
      window.removeEventListener("drop", end);
    };
  }, []);
  const rows = (focus?.pins ?? []).flatMap((pin) => {
    const r = refNamed(pin.name, known);
    return r ? [{ pin, r }] : [];
  });
  const bringing = incoming && focus?.supported && !focus.pinned(incoming) ? incoming : undefined;
  if (!focus || (rows.length === 0 && !bringing)) return null;

  const names = rows.map((x) => x.pin.name);
  const reset = () => {
    setMoving(undefined);
    setIncoming(undefined);
    setOver(undefined);
  };
  // where is the pin a drop goes ahead of: the row it's over, or the one
  // after it; undefined for the end.
  const ahead = () => (over ? (over.after ? names[names.indexOf(over.name) + 1] : over.name) : undefined);
  const drop = (e: DragEvent) => {
    e.preventDefault();
    if (bringing) {
      focus.pin(bringing, ahead());
    } else if (moving && over && over.name !== moving) {
      const rest = names.filter((n) => n !== moving);
      const before = ahead();
      const at = before && before !== moving ? rest.indexOf(before) : rest.length;
      rest.splice(at < 0 ? rest.length : at, 0, moving);
      if (rest.join("\n") !== names.join("\n")) focus.arrange(rest);
    }
    reset();
  };
  const dragOver = (e: DragEvent) => {
    if (!moving && !bringing) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = bringing ? "copy" : "move";
  };

  return (
    <Focus count={rows.length} bringing={!!bringing} onDragOver={dragOver} onDrop={drop}>
      {rows.map(({ pin, r }) => (
        <PinRow
          key={pin.name}
          pin={pin}
          r={r}
          known={known}
          path={path}
          dragging={moving === pin.name}
          drop={over?.name === pin.name && moving !== pin.name ? (over.after ? "after" : "before") : undefined}
          onUnpin={() => focus.unpin(pin.name)}
          drag={{
            onDragStart: (e) => {
              e.dataTransfer.effectAllowed = "move";
              e.dataTransfer.setData("text/plain", pin.name);
              setMoving(pin.name);
            },
            onDragOver: (e) => {
              if (!moving && !bringing) return;
              const box = e.currentTarget.getBoundingClientRect();
              setOver({ name: pin.name, after: e.clientY > box.top + box.height / 2 });
            },
            onDragEnd: reset,
          }}
        />
      ))}
    </Focus>
  );
}
