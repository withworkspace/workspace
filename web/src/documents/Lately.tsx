import type { ReactNode } from "react";
import type { Space, SpaceCommit } from "../api";
import { ChangeLine } from "./ChangeLine";
import { DocumentsActivitySection } from "./DocumentsActivitySection";
import { DocumentsChanges } from "./DocumentsChanges";
import type { InSpace } from "./happening";

// Lately is the commits on the default branch lately that changed
// documents (a space's, or every space's, each with its space), a line
// each: which document, what was done to it, who and when. flat: in a
// space's DocumentsActivityFold.
export function Lately({ lately, limit = 5, spaceOf, label = "Recent changes", flat }: { lately: InSpace<SpaceCommit>[]; limit?: number; spaceOf?: (repo: string) => Space | undefined; label?: ReactNode; flat?: boolean }) {
  if (!lately.length) return null;
  const shown = lately.slice(0, limit);
  const earlier = lately.length - shown.length;
  return (
    <DocumentsActivitySection label="Recent changes" heading={label} count={lately.length} flat={flat}>
      <DocumentsChanges more={earlier > 0 && `and ${earlier === 1 ? "1 earlier change" : `${earlier} earlier changes`}`}>
        {shown.map((c) => (
          <ChangeLine key={`${c.repo}:${c.sha}`} c={c} space={spaceOf?.(c.repo)} />
        ))}
      </DocumentsChanges>
    </DocumentsActivitySection>
  );
}
