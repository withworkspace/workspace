import { Chip } from "../ui/Chip";
import type { Placed } from "./place";

// DraftNote is where a draft of yours in a space stands, beside it in the
// space's list: not filed yet (and where it will be), unsaved, or filed
// in a capsule and not landed.
export function DraftNote({ p }: { p: Placed }) {
  switch (p.why) {
    case "to":
      return (
        <Chip tone="quiet" title={`Will be filed in ${p.path || "its top"}`}>
          Not filed · {p.path || "top"}
        </Chip>
      );
    case "unsaved":
      return (
        <Chip tone="attention" title={`Unsaved edits to ${p.path}`}>
          Unsaved
        </Chip>
      );
    case "filed":
      return (
        <Chip tone="progress" title={`Filed as ${p.path}: shows here once landed`}>
          Filed, not landed
        </Chip>
      );
    default:
      return (
        <Chip tone="quiet" title="Started from a document kind, not filed yet">
          Not filed
        </Chip>
      );
  }
}
