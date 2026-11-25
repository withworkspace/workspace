import type { ComponentProps } from "react";
import { Empty } from "../../ui/Empty";
import css from "./MissionWorkEmpty.module.css";

// MissionWorkEmpty says there's nothing to show of what's open in the
// Work tab (a capsule nothing's changed in, an output that's read
// elsewhere), with room around it.
export function MissionWorkEmpty(props: Omit<ComponentProps<typeof Empty>, "className">) {
  return <Empty className={css.empty} {...props} />;
}
