import { useEffect, useState, useSyncExternalStore } from "react";
import { getTalk, settleTalkProposal, type TalkDetail } from "../api";
import { ProposalCard } from "../talk/ProposalCard";
import { Dialog, DialogText } from "../ui/Dialog";
import { ErrorCard } from "../ui/ErrorCard";
import { Skeleton } from "../ui/Skeleton";
import { humanError } from "../ui/strings";
import { actAnswered, actAsked, onActAsked } from "./confirmAct";

// ActConfirmer draws what confirmAct asks: once, by the app. It's the
// proposal's own card, as its conversation has it, in a dialog: accept it
// (all, or the items ticked) and it's done; decline it and it isn't. The
// run that made it goes on either way, told which.
export function ActConfirmer() {
  const ask = useSyncExternalStore(onActAsked, actAsked);
  if (!ask) return null;
  return <Confirming key={`${ask.conversation}#${ask.n}`} conversation={ask.conversation} n={ask.n} title={ask.title} />;
}

function Confirming({ conversation, n, title }: { conversation: string; n: number; title: string }) {
  const [talk, setTalk] = useState<TalkDetail>();
  const [error, setError] = useState<string>();
  useEffect(() => {
    getTalk("", conversation).then(setTalk, (e: Error) => setError(humanError(e.message)));
  }, [conversation]);
  const p = talk?.proposals?.find((x) => x.n === n);
  return (
    <Dialog title={title} onClose={() => actAnswered(undefined)} wide>
      <DialogText>Your automation does this through its plugin once you say so. It's kept in its conversation, {conversation}, either way.</DialogText>
      {error && <ErrorCard error={error} />}
      {!p && !error && <Skeleton rows={3} />}
      {p && (
        <ProposalCard
          p={p}
          onSettle={async (num, action, edits) => {
            try {
              await settleTalkProposal("", conversation, num, action, (edits ?? {}) as { items?: number[] });
              actAnswered(action === "accept" ? "Done." : "Declined: it isn't done.");
            } catch (e) {
              setError(humanError((e as Error).message));
            }
          }}
        />
      )}
    </Dialog>
  );
}
