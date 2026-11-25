import type { V1 } from "@withworkspace/ui-lib/protocol";
import { type ReactNode, type RefObject, useMemo } from "react";
import { anchorIn, type Asked, type FrameUis } from "../ui";
import { RefCardUi } from "./RefCardUi";
import { ThingPromoteUi } from "./ThingPromoteUi";
import { ThingSaveUi } from "./ThingSaveUi";
import type { UiProps } from "./uiProps";

// What draws each, by its name in the protocol.
const draws: Readonly<Record<V1.UiName, (props: UiProps) => ReactNode>> = {
  "ref-card": RefCardUi,
  "thing-promote": ThingPromoteUi,
  "thing-save": ThingSaveUi,
};

// FrameUi is one UI a plugin's page asked for, drawn by what draws its
// kind, placed by where on the page it was asked from.
export function FrameUi({ asked, uis, frame, plugin }: { asked: Asked; uis: FrameUis; frame: RefObject<Element | null>; plugin: string }) {
  const anchor = useMemo(() => asked.box && anchorIn(() => frame.current, asked.box), [asked, frame]);
  const Draw = Object.hasOwn(draws, asked.ui) ? draws[asked.ui as V1.UiName] : undefined;
  if (!Draw) return null;
  return <Draw asked={asked} plugin={plugin} anchor={anchor} done={(result) => uis.done(asked.id, result)} stay={uis.stay} leave={uis.leave} />;
}
