import type { IconName } from "../ui/Icon";
import type { Said } from "../api";

// What someone did, said on GitHub, in words and as an icon.
export const saidWords: Record<Said["kind"], string> = {
  comment: "commented",
  approved: "approved",
  changes: "asked for changes",
  commented: "reviewed",
  dismissed: "had a review dismissed",
};

export const saidIcon = (k: Said["kind"]): IconName => (k === "approved" ? "check-circle" : k === "changes" ? "alert" : "message");
