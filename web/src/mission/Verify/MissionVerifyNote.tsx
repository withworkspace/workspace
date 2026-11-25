import type { ComponentProps } from "react";
import { Callout } from "../../ui/Callout";
import css from "./MissionVerifyNote.module.css";

// MissionVerifyNote is something to know of the open bundle, in its head:
// that another has taken its place, that you approved it or sent it back,
// that it's stale. A Callout, with no room under it: the head spaces it.
export function MissionVerifyNote(props: Omit<ComponentProps<typeof Callout>, "className">) {
  return <Callout className={css.note} {...props} />;
}
