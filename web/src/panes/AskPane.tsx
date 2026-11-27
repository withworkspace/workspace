import { useEffect, useRef } from "react";
import { DockBody } from "../chrome/DockBody";
import { PaneHead } from "../chrome/PaneHead";
import { useChromeStore, useGives, useKeysAsked } from "../chrome/store";
import { Icon } from "../ui/Icon";
import { Composer } from "./Composer";
import css from "./AskPane.module.css";

type AskPaneProps = {
  // Its tab's address (chrome/panes.ts's askAt), and the thing it's about.
  to: string;
  thing: string;
};

// AskPane is a conversation about a thing that hasn't begun: open beside
// the thing's page, with what the page offers asking and a place to ask.
// Asking begins it, and it's a conversation from then (TalkPane), in this
// tab's place. It's the page's: how asking begins is the page's to say,
// so it's there only while the page is.
export function AskPane({ to, thing }: AskPaneProps) {
  const gives = useGives(to);
  const field = useRef<HTMLElement>(null);
  const asked = useKeysAsked(to);
  useEffect(() => {
    if (asked && useChromeStore.getState().takeKeys(to)) field.current?.focus();
  }, [asked, to]);

  // Asking: the page begins it, and says why if it couldn't.
  const start = (q: string) => gives?.start?.(q) ?? Promise.reject(new Error("The page it's about has gone."));
  return (
    <>
      <DockBody label={`Ask about ${thing}`} pad>
        <PaneHead title={`Ask about ${thing}`} />
        <p className={css.says}>Ask about {thing}, or request a change.</p>
        <div className={css.offered}>
          {(gives?.suggestions ?? []).map((q) => (
            <button key={q} type="button" className="ask-suggestion" onClick={() => void start(q).catch(() => {})}>
              <Icon name="sparkle" size={11} />
              {q}
            </button>
          ))}
        </div>
      </DockBody>
      <Composer field={field} placeholder={gives?.placeholder ?? `Ask about ${thing}…`} holding={gives?.holding} onSay={start} />
    </>
  );
}
