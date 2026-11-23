import type { V1 } from "@withworkspace/ui-lib/protocol";
import { type ReactNode, type RefObject, useMemo, useSyncExternalStore } from "react";
import { useWorkspaceIfRead } from "../workspace/store";
import { RefCard } from "../refs/RefCard";
import { refOf } from "../refs/refs";
import type { Anchor } from "../ui/FloatingCard";
import { ThingPromoteUi, ThingSaveUi } from "./ThingUis";
import { anchorIn, type Asked, type FrameUis } from "./ui";

// What ws draws for each of its own UIs a plugin's page asked for
// (ui.ts), over the page's frame. Each is ws's own component, drawn in
// ws's page, so it sits over the frame and past its edge. It's placed by
// where on the app's page it was asked from: the box the app gave, from
// the frame's corner.

// UiProps is what one is drawn with.
export interface UiProps {
  // What the page asked for: what it's given is as its kind takes it.
  asked: Asked;
  // The plugin whose page asked.
  plugin: string;
  // Where it was asked from, in ws's window, if the page said.
  anchor?: Anchor;
  // ws is done with it: it goes, and the page is told what came of it.
  done(result?: unknown): void;
  // On hover: the pointer's on it, and has left it.
  stay(): void;
  leave(): void;
}

// A ref's card, by the ref's words in the app's page: the card ws shows
// for a ref in its own pages.
function RefCardUi({ asked, anchor, stay, leave }: UiProps) {
  const workspace = useWorkspaceIfRead();
  const r = typeof asked.with.ref === "string" ? refOf(asked.with.ref) : undefined;
  // A ref ws can't read has no card.
  if (!r || !anchor || !workspace) return null;
  return <RefCard r={r} el={anchor} onEnter={stay} onLeave={leave} hint={false} />;
}

// What draws each, by its name in the protocol.
const draws: Readonly<Record<V1.UiName, (props: UiProps) => ReactNode>> = {
  "ref-card": RefCardUi,
  "thing-promote": ThingPromoteUi,
  "thing-save": ThingSaveUi,
};

function Drawn({ asked, uis, frame, plugin }: { asked: Asked; uis: FrameUis; frame: RefObject<Element | null>; plugin: string }) {
  const anchor = useMemo(() => asked.box && anchorIn(() => frame.current, asked.box), [asked, frame]);
  const Draw = Object.hasOwn(draws, asked.ui) ? draws[asked.ui as V1.UiName] : undefined;
  if (!Draw) return null;
  return <Draw asked={asked} plugin={plugin} anchor={anchor} done={(result) => uis.done(asked.id, result)} stay={uis.stay} leave={uis.leave} />;
}

// FrameUiLayer is what a plugin's page has asked ws to show, drawn over
// its frame: what's shown on hover, and whatever's open.
export function FrameUiLayer({ uis, frame, plugin }: { uis: FrameUis; frame: RefObject<Element | null>; plugin: string }) {
  const hovered = useSyncExternalStore(uis.hear, uis.hovered);
  const opened = useSyncExternalStore(uis.hear, uis.opened);
  return (
    <>
      {hovered && <Drawn key={`hover-${hovered.id}`} asked={hovered} uis={uis} frame={frame} plugin={plugin} />}
      {opened.map((asked) => (
        <Drawn key={asked.id} asked={asked} uis={uis} frame={frame} plugin={plugin} />
      ))}
    </>
  );
}
