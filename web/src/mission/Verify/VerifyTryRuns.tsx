import { createContext, useContext } from "react";
import type { CapsuleDetail } from "../../api";
import type { Reload } from "../../load";

// TryRuns is what a bundle's Try it needs to start the recipes it names:
// the mission, the bundle's capsule as the mission has it (its recipes
// and what's running in it; none when it's gone), whether the capsule has
// moved on since the bundle (moved), and how to reload once one's asked
// for.
export type TryRuns = {
  slug: string;
  capsule?: CapsuleDetail;
  moved?: boolean;
  reload: Reload;
};

const TryRunsContext = createContext<TryRuns | undefined>(undefined);

// VerifyTryRuns gives the open bundle's TryRuns to the tries under it.
export const VerifyTryRuns = TryRunsContext.Provider;

// useTryRuns is the open bundle's TryRuns, where there's one.
export const useTryRuns = () => useContext(TryRunsContext);
