import { useRef } from "react";
import { Banner } from "../ui/Banner";
import { Button } from "../ui/Button";
import { SendKeys } from "../ui/SendKeys";
import { sendKeys, sent, useSendKey } from "../ui/sendKey";
import { humanError } from "../ui/text";
import { TextArea } from "../ui/TextArea";
import { useAutosize } from "../ui/useAutosize";
import css from "./SaidReply.module.css";

// SaidReply is you commenting on a pull request itself: what you're
// writing, sent with the key you've chosen (ui/sendKey.ts), and that it
// posts to GitHub at once, as you, not as part of your draft.
export function SaidReply({ text, busy, error, onText, onSend }: { text: string; busy: boolean; error?: string; onText: (t: string) => void; onSend: () => void }) {
  const area = useRef<HTMLTextAreaElement>(null);
  const sendKey = useSendKey();
  useAutosize(area, text, 240);
  return (
    <div className={css.reply} onClick={(e) => e.stopPropagation()}>
      <TextArea
        ref={area}
        value={text}
        rows={2}
        placeholder="Comment on the pull request…"
        aria-label="Comment on the pull request"
        onChange={(e) => onText(e.target.value)}
        onKeyDown={(e) => {
          if (e.nativeEvent.isComposing) return;
          if (sent(e, sendKey)) {
            e.preventDefault();
            onSend();
          } else if (e.key === "Escape") {
            e.preventDefault();
            e.currentTarget.blur();
          }
        }}
      />
      {error && <Banner>{humanError(error)}</Banner>}
      <div className={css.actions}>
        <span className="dim">Posts to GitHub at once, as you: it isn't part of your draft.</span>
        <span className={css.spacer} />
        <Button variant="primary" size="small" disabled={!text.trim() || busy} onClick={onSend} title="Post it to GitHub" keys={sendKeys(sendKey)}>
          {busy ? "Posting…" : "Comment on GitHub"} <SendKeys />
        </Button>
      </div>
    </div>
  );
}
