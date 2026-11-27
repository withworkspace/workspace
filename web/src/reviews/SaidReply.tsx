import { Banner } from "../ui/Banner";
import { Button } from "../ui/Button";
import { SendKeys } from "../ui/SendKeys";
import { sendKeys, useSendKey } from "../ui/sendKey";
import { humanError } from "../ui/strings";
import css from "./SaidReply.module.css";
import { Text } from "../ui/Text";
import { RichTextArea } from "../ui/RichTextArea";

// SaidReply is you commenting on a pull request itself: what you're
// writing, sent with the key you've chosen (ui/sendKey.ts), and that it
// posts to GitHub at once, as you, not as part of your draft.
export function SaidReply({ text, busy, error, onText, onSend }: { text: string; busy: boolean; error?: string; onText: (t: string) => void; onSend: () => void }) {
  const sendKey = useSendKey();
  return (
    <div className={css.reply} onClick={(e) => e.stopPropagation()}>
      <RichTextArea
        marks
        lists
        quote
        code
        refs
        look="field"
        value={text}
        rows={2}
        tallest={240}
        placeholder="Comment on the pull request…"
        aria-label="Comment on the pull request"
        onChange={onText}
        send={sendKey}
        onSend={onSend}
        onEscape={() => (document.activeElement as HTMLElement | null)?.blur()}
      />
      {error && <Banner>{humanError(error)}</Banner>}
      <div className={css.actions}>
        <Text dim>Posts to GitHub immediately, not part of your draft.</Text>
        <span className={css.spacer} />
        <Button variant="primary" size="small" disabled={!text.trim() || busy} onClick={onSend} title="Post to GitHub" keys={sendKeys(sendKey)}>
          {busy ? "Posting…" : "Comment on GitHub"} <SendKeys />
        </Button>
      </div>
    </div>
  );
}
