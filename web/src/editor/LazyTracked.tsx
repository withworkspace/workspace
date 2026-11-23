import { lazy, Suspense } from "react";
import type { TrackedProps } from "./TrackedDoc";
import { Skeleton } from "../ui/Skeleton";

export type { TrackedChange, TrackedProps } from "./TrackedDoc";

// The tracked view loads with the editor's code, the first time it's
// asked for.
const Inner = lazy(() => import("./TrackedDoc"));

export function TrackedDoc(props: TrackedProps) {
  return (
    <Suspense fallback={<Skeleton rows={10} />}>
      <Inner {...props} />
    </Suspense>
  );
}
