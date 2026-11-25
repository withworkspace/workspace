import type { ComponentProps } from "react";
import { Empty } from "../ui/Empty";
import css from "./ArtifactEmpty.module.css";

// ArtifactEmpty is what's said where an artifact's frame would be (it's
// stopped, there's no such version, its files have gone): an Empty, a way
// down its room.
export function ArtifactEmpty(props: Omit<ComponentProps<typeof Empty>, "className" | "big" | "done">) {
  return <Empty {...props} className={css.empty} />;
}
