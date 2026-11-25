import { useState } from "react";
import type { Talk } from "../api";
import { TalkList } from "../talk/TalkList";

// TalkTab is the mission's Talk tab: conversations about where it's going
// (its plan, what next, what was decided) while crew get on with it.
export function TalkTab({ slug, talks, planner }: { slug: string; talks: Talk[]; planner?: string }) {
  const [showResolved, setShowResolved] = useState(false);
  return (
    <div>
      <TalkList slug={slug} talks={talks} planner={planner} showResolved={showResolved} onToggleResolved={() => setShowResolved((v) => !v)} />
    </div>
  );
}
