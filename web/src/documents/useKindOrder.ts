import { useState } from "react";
import type { Sort } from "./sorted";

// useKindOrder is how each kind's list is sorted and filtered, as chosen.
export function useKindOrder() {
  const [sorts, setSorts] = useState<Record<string, Sort>>({});
  const [stands, setStands] = useState<Record<string, string>>({});
  return {
    sortOf: (k: string) => sorts[k] ?? "made",
    setSort: (k: string, s: Sort) => setSorts((x) => ({ ...x, [k]: s })),
    standOf: (k: string) => stands[k] ?? "",
    setStand: (k: string, s: string) => setStands((x) => ({ ...x, [k]: s })),
  };
}
