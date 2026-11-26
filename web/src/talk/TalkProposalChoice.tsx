import { Button } from "../ui/Button";
import { Spinner } from "../ui/Spinner";
import css from "./TalkProposalChoice.module.css";
import { Text } from "../ui/Text";

// TalkProposalChoice is what you can do with a proposal that's open:
// decline it, or accept it (accept says what accepting does). busy is
// what's being done, while it is; ready, whether it can be accepted as it
// stands; replying, the agent's still writing the reply it came with, so
// it's read first.
export function TalkProposalChoice({
  busy,
  accept,
  decline = "Decline",
  declineTitle,
  ready,
  replying,
  onAccept,
  onDecline,
}: {
  busy?: string;
  accept: string;
  decline?: string;
  declineTitle?: string;
  ready: boolean;
  replying?: boolean;
  onAccept: () => void;
  onDecline: () => void;
}) {
  return (
    <>
      {busy && (
        <Text dim className={css.busy}>
          <Spinner /> {busy}
        </Text>
      )}
      <span className={css.spacer} />
      <Button variant="ghost" disabled={!!busy} onClick={onDecline} title={declineTitle}>
        {decline}
      </Button>
      <Button variant="primary" disabled={!!busy || !ready} unavailable={replying ? "Read its reply first: the agent's still writing it" : undefined} onClick={onAccept}>
        {accept}
      </Button>
    </>
  );
}
