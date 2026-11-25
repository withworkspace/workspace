import { type RefObject, useSyncExternalStore } from "react";
import type { FrameUis } from "../ui";
import { FrameUi } from "./FrameUi";

// What ws draws for each of its own UIs a plugin's page asked for
// (ui.ts), over the page's frame. Each is ws's own component, drawn in
// ws's page, so it sits over the frame and past its edge. It's placed by
// where on the app's page it was asked from: the box the app gave, from
// the frame's corner.

// FrameUiLayer is what a plugin's page has asked ws to show, drawn over
// its frame: what's shown on hover, and whatever's open.
export function FrameUiLayer({ uis, frame, plugin }: { uis: FrameUis; frame: RefObject<Element | null>; plugin: string }) {
  const hovered = useSyncExternalStore(uis.hear, uis.hovered);
  const opened = useSyncExternalStore(uis.hear, uis.opened);
  return (
    <>
      {hovered && <FrameUi key={`hover-${hovered.id}`} asked={hovered} uis={uis} frame={frame} plugin={plugin} />}
      {opened.map((asked) => (
        <FrameUi key={asked.id} asked={asked} uis={uis} frame={frame} plugin={plugin} />
      ))}
    </>
  );
}
