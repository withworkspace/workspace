import { useEffect, useRef } from "react";
import { startTalk } from "../api";
import { DockBody } from "../chrome/DockBody";
import { PaneHead } from "../chrome/PaneHead";
import { talkAt } from "../chrome/panes";
import { useChromeStore, useKeysAsked } from "../chrome/store";
import { useTalks } from "../conversation/talks";
import { Keys } from "../ui/Keys";
import { humanError } from "../ui/text";
import { toast } from "../ui/toast";
import { useDraft } from "../ui/useDraft";
import { Composer } from "./Composer";
import css from "./NewTalkPane.module.css";

type NewTalkPaneProps = {
  // Its tab's address (chrome/panes.ts's newTalkAt).
  to: string;
};

// NewTalkPane is a conversation about to begin, beside whatever page
// you're on (C): a place to say the first thing, and nothing else. ws
// has no conversation for it yet, so closing its tab leaves nothing
// behind; saying something begins one, which is a conversation from then
// (TalkPane), in this tab's place. What's written and not yet said is
// the same draft as the new conversation's on Conversations, so it's
// there whichever you come back to.
export function NewTalkPane({ to }: NewTalkPaneProps) {
  const [text, setText] = useDraft("new:");
  const field = useRef<HTMLTextAreaElement>(null);
  const asked = useKeysAsked(to);
  useEffect(() => {
    if (asked && useChromeStore.getState().takeKeys(to)) field.current?.focus();
  }, [asked, to]);

  const begin = (body: string) =>
    startTalk("", body).then(
      (d) => {
        const at = talkAt(d.id);
        useTalks.getState().told(at, d);
        useChromeStore.getState().open(at, { inPlaceOf: to, keys: true });
      },
      (e: Error) => {
        toast(humanError(e.message), { tone: "danger" });
        throw e;
      },
    );
  return (
    <>
      <DockBody label="New conversation" pad>
        <PaneHead title="New conversation" />
        <p className={css.says}>What are you thinking about? An idea, a problem, a question about the code.</p>
        <p className={css.kept}>
          Nothing's kept until you say something (<Keys k="mod+⏎" />): then it's a conversation, in Conversations.
        </p>
      </DockBody>
      <Composer field={field} placeholder="Say the first thing…" text={text} onText={setText} onSay={begin} />
    </>
  );
}
