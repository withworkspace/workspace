import { Chip } from "../ui/Chip";
import type { Placed } from "./place";

// DraftNote is where a draft of yours in a space stands, beside it in the
// space's list: not filed yet (and where it will be), unsaved, or filed
// in a capsule and not landed.
export function DraftNote({ p }: { p: Placed }) {
  switch (p.why) {
    case "to":
      return (
        <Chip tone="quiet" title={`To be filed in ${p.path || "its top"}, not filed yet`}>
          Not filed · {p.path || "top"}
        </Chip>
      );
    case "unsaved":
      return (
        <Chip tone="attention" title={`Your edits to ${p.path}, not saved to the repo yet`}>
          Unsaved
        </Chip>
      );
    case "filed":
      return (
        <Chip tone="progress" title={`Filed as ${p.path}, in a capsule: it's here once it lands`}>
          Filed, not landed
        </Chip>
      );
    default:
      return (
        <Chip tone="quiet" title="Started as one of this space's kinds, not filed yet">
          Not filed
        </Chip>
      );
  }
}
