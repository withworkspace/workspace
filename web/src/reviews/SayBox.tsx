import { useState } from "react";
import { commentPR, type Discussion } from "../api";
import { SaidReply } from "./SaidReply";

// SayBox is you commenting on the pull request itself. It posts at once,
// as a reply in a thread does, and says so.
export function SayBox({ api, onSaid }: { api: string; onSaid: (d: Discussion) => void }) {
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const send = () => {
    if (!text.trim() || busy) return;
    setBusy(true);
    commentPR(api, text)
      .then(
        (d) => {
          setText("");
          setError(undefined);
          onSaid(d);
        },
        (e: Error) => setError(e.message),
      )
      .finally(() => setBusy(false));
  };
  return <SaidReply text={text} busy={busy} error={error} onText={setText} onSend={send} />;
}
