import { useState } from "react";
import { Button } from "../ui/Button";
import { Keys } from "../ui/Keys";
import { PageHead } from "../ui/PageHead";
import { useTitle } from "../ui/useTitle";
import { useWorkspace } from "../workspace/store";
import { TalkList } from "./TalkList";

// Conversations with an agent about the workspace: thinking something
// through before there's a mission. The agent reads anything (the repos,
// the missions, GitHub) and changes nothing; it answers in a run of its
// own, whose steps show as it goes. TalksPage is the workspace's, listed.
export function TalksPage() {
  const conversations = useWorkspace((w) => w.conversations);
  const kinds = useWorkspace((w) => w.kinds);
  useTitle("Conversations");
  const [showResolved, setShowResolved] = useState(false);
  const resolved = conversations.filter((t) => t.status !== "open").length;
  return (
    <div className="page narrow">
      <PageHead
        title="Conversations"
        actions={
          resolved > 0 && (
            <>
              <Button variant="ghost" size="small" onClick={() => setShowResolved((v) => !v)} keys="⇧A">
                {showResolved ? "Hide resolved" : `Show resolved (${resolved})`} <Keys k="⇧A" />
              </Button>
            </>
          )
        }
      />
      <TalkList slug="" talks={conversations} showResolved={showResolved} onToggleResolved={() => setShowResolved((v) => !v)} keys docs={!!kinds?.length} />
    </div>
  );
}
