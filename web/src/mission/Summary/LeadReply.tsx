import { useState } from "react";
import { sayToCrew } from "../../api";
import type { Reload } from "../../load";
import { Button } from "../../ui/Button";
import { Input } from "../../ui/Input";
import { MoreLink } from "../../ui/MoreLink";
import { humanError } from "../../ui/text";
import { toast } from "../../ui/toast";
import { MissionLeadAnswer, MissionLeadAnswerForm } from "./MissionLeadAnswer";

// LeadReply answers crew that are waiting, from the mission's page.
export function LeadReply({ slug, id, reload }: { slug: string; id: string; reload: Reload }) {
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const send = () => {
    if (!text.trim()) return;
    setBusy(true);
    sayToCrew(slug, id, text)
      .then(
        (r) => {
          setText("");
          toast(r.message, { tone: "done" });
          return reload();
        },
        (e: Error) => toast(humanError(e.message), { tone: "danger" }),
      )
      .finally(() => setBusy(false));
  };
  return (
    <MissionLeadAnswer>
      <MissionLeadAnswerForm onSubmit={send}>
        <Input value={text} onChange={(e) => setText(e.target.value)} placeholder={`Reply to ${id}…`} aria-label={`Reply to ${id}`} disabled={busy} />
        <Button variant="primary" type="submit" disabled={busy || !text.trim()}>
          Send
        </Button>
      </MissionLeadAnswerForm>
      <MoreLink to={`/missions/${slug}/crew/${id}`}>
        Open {id}'s conversation
      </MoreLink>
    </MissionLeadAnswer>
  );
}
