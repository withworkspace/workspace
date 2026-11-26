import { type Ref, useState } from "react";
import { answerCrew, type Crew, sayToCrew } from "../../api";
import { Button } from "../../ui/Button";
import { Composer } from "../../ui/Composer";
import { Icon } from "../../ui/Icon";
import { sendKeys, useSendKey } from "../../ui/sendKey";
import { humanError } from "../../ui/strings";
import { toast } from "../../ui/toast";
import { useDraft } from "../../ui/useDraft";

// CrewTell tells crew what's next: typed in and sent if it's running, or
// resuming it with it if it's ended. While it works, Interrupt stops its
// turn (Escape), not the crew. field: the box, for whoever gives it the
// keys (the crew's page, on r).
export function CrewTell({ slug, id, crew, live, working, field }: { slug: string; id: string; crew?: Crew; live: boolean; working: boolean; field?: Ref<HTMLTextAreaElement> }) {
  const [text, setText] = useDraft(`crew:${slug}/${id}`);
  const [busy, setBusy] = useState(false);
  const canResume = !live && !!crew && crew.ended;
  // The key that sends, as the box has it (ui/Composer.tsx): the button's too.
  const sendKey = useSendKey();
  const send = () => {
    if (!text.trim() || busy) return;
    setBusy(true);
    sayToCrew(slug, id, text)
      .then(
        (r) => {
          setText("");
          toast(r.message, { tone: "done" });
        },
        (e: Error) => toast(humanError(e.message), { tone: "danger" }),
      )
      .finally(() => setBusy(false));
  };
  const interrupt = () =>
    answerCrew(slug, id, { interrupt: true }).then(
      (r) => toast(r.message),
      (e: Error) => toast(humanError(e.message), { tone: "danger" }),
    );
  if (!live && !canResume) return null;
  return (
    <Composer
      field={field}
      value={text}
      onChange={setText}
      onSend={send}
      tallest={280}
      aria-label={`Message ${id}`}
      placeholder={live ? (working ? `Message ${id}: read after this step` : `Tell ${id} what's next…`) : `Resume ${id} with a message…`}
      hint={
        live && crew?.structured ? (
          <>
            <Icon name="message" size={12} /> Sent as a message{working ? ", for when this turn ends" : ""}.
          </>
        ) : live ? (
          <>
            <Icon name="terminal" size={12} /> Typed into the terminal.
          </>
        ) : (
          <>
            <Icon name="rotate" size={12} /> Ended: sending resumes with your message.
          </>
        )
      }
      also={
        working && (
          <Button variant="ghost" onClick={interrupt} title="Stop current turn">
            <Icon name="stop" size={12} /> Interrupt
          </Button>
        )
      }
    >
      <Button variant="primary" disabled={!text.trim() || busy} onClick={send} keys={sendKeys(sendKey)}>
        {busy ? "Sending…" : live ? "Send" : "Resume"}
      </Button>
    </Composer>
  );
}
