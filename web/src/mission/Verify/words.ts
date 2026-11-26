import type { Bundle } from "../../api";
import type { IconName } from "../../ui/Icon";
import { waiting } from "../verifyRows";

// What the Verify tab says in words of a bundle, and how.

// verdictIcon is the icon of what a check came to.
export const verdictIcon: Record<string, IconName> = { met: "check-circle", unmet: "x-circle", "can't tell": "question", yours: "user" };

// settles is what approving a bundle does, and that it's stale where it is.
export function settles(b: Bundle): string {
  return `${b.asked ? `Approving ${b.id} passes step ${b.askedAt}.` : `No step waits on ${b.id}: your verdict is saved.`}${b.stale ? " Stale: capsule has moved on since." : ""}`;
}

// stepTitle is what a bundle is of, as its step is listed.
export function stepTitle(b: Bundle): string {
  if (b.step && b.stepName) return `Step ${b.step}: ${b.stepName}`;
  return b.capsule ? `Of ${b.capsule}` : "Of no step";
}

// standingTone is the tone a bundle's standing is said in, if any.
export function standingTone(b: Bundle): "done" | "attention" | undefined {
  if (b.replacedBy) return undefined;
  if (b.verdict === "approved") return b.stale ? undefined : "done";
  if (waiting(b) || b.verdict === "sent back" || b.stale) return "attention";
  return undefined;
}
