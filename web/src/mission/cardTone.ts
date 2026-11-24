import type { MissionCard } from "../api";
import type { CalloutTone } from "../ui/Callout";

// A card's tone is whose it is: yours, failing (and yours), an agent's,
// outside's, done, or nobody's in particular.
export const cardTone: Record<MissionCard["tone"], CalloutTone> = { you: "attention", failing: "danger", agent: "progress", outside: "info", done: "done", quiet: "quiet" };
