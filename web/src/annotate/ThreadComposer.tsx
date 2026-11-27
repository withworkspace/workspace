import { useEffect, useRef, useState } from "react";
import { threadKinds, type ThreadKind } from "../api";
import { useCommands } from "../keymap/useCommands";
import { annotateKeys } from "./keys";
import { sendKeys, useSendKey } from "../ui/sendKey";
import { clip } from "../ui/strings";
import { AnnotateComposer } from "./AnnotateComposer";
import type { Pick } from "./geometry";

// ThreadComposer writes a new thread on what's highlighted: tab picks ask,
// change or note; the key you've chosen posts it (ui/sendKey.ts), and
// it's live at once. In an edit of your own, ⏎ is a new line of it, and
// ⌘⏎ posts.
export function ThreadComposer({ pick, onPost, onCancel }: { pick: Pick; onPost: (kind: ThreadKind, body: string, suggestion?: string) => Promise<void>; onCancel: () => void }) {
  const [kind, setKind] = useState<ThreadKind>("ask");
  const [text, setText] = useState("");
  // An edit of your own: what the highlighted text should read.
  const [edit, setEdit] = useState<string>();
  const [error, setError] = useState<string>();
  const area = useRef<HTMLElement>(null);
  const sendKey = useSendKey();
  // While writing, the page's keys are off: these are the composer's. (Its
  // slot is see-through, not hidden, until it's placed: focus needs that.)
  useCommands(annotateKeys, {}, { modal: true });
  useEffect(() => {
    area.current?.focus({ preventScroll: true });
  }, []);
  const post = () => {
    if (edit !== undefined) {
      if (edit === pick.quote.exact) return;
      onPost("change", text.trim() || "This should read differently.", edit).catch((e: Error) => setError(e.message));
      return;
    }
    if (!text.trim()) return;
    onPost(kind, text).catch((e: Error) => setError(e.message));
  };
  const suggest = () => {
    setKind("change");
    setEdit(pick.quote.exact);
  };
  return (
    <AnnotateComposer
      kind={kind}
      onKind={setKind}
      quote={pick.quote.exact ? clip(pick.quote.exact.replace(/\s+/g, " "), 140) : undefined}
      text={text}
      onText={setText}
      textRef={area}
      onKeys={(e) => {
        if (e.key !== "Tab" || e.metaKey || e.ctrlKey || e.altKey) return false;
        const i = threadKinds.indexOf(kind);
        setKind(threadKinds[(i + (e.shiftKey ? threadKinds.length - 1 : 1)) % threadKinds.length]);
        return true;
      }}
      edit={edit}
      onEdit={setEdit}
      onEditKeys={(e) => {
        if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
          e.preventDefault();
          post();
        } else if (e.key === "Escape") {
          e.preventDefault();
          onCancel();
        }
      }}
      error={error}
      canSuggest={!!pick.quote.exact}
      onSuggest={suggest}
      canPost={edit !== undefined ? edit !== pick.quote.exact : !!text.trim()}
      onPost={post}
      postKeys={sendKeys(sendKey)}
      onCancel={onCancel}
    />
  );
}
