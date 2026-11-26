import { Empty } from "../ui/Empty";
import { type Shown, shownLabels } from "./shown";

// ArtifactsNone is what the Artifacts page says in place of its list:
// that there are none yet, and what one is; or, with a filter on (shown),
// that none of those came from where's picked.
export function ArtifactsNone({ shown }: { shown?: Shown }) {
  if (!shown) {
    return (
      <Empty icon="artifact" title="No artifacts yet">
        Artifacts are versioned prototype pages to look at and use. Crew add them with{" "}
        <code>ws artifact add</code>, and so can you.
      </Empty>
    );
  }
  return (
    <Empty icon="search" title="None here">
      No artifact {shown === "all" ? "" : `that's ${shownLabels[shown].toLowerCase()} `}came from there.
    </Empty>
  );
}
