import { useEffect, useRef, useState } from "react";
import { Button } from "../ui/Button";
import { SendKeys } from "../ui/SendKeys";
import { sendKeys, useSendKey } from "../ui/sendKey";
import { PageComposer } from "./PageComposer";
import { PagePanel } from "./PagePanel";
import { PagePanelFoot } from "./PagePanelFoot";

// ChangeBox asks what should change in words of yours, under them: asked,
// they become a cell, and an agent writes them again. (Or what's to be
// added to the page, or a question about the words selected.)
export function ChangeBox({
  top,
  onClose,
  onAsk,
  placeholder = "What should change here? Tighten it, make it a table, add what's missing…",
  hint = "Becomes a cell for an agent to rewrite. Your original is kept.",
  label = "Ask for a change",
}: {
  top: number;
  onClose: () => void;
  onAsk: (prompt: string) => void;
  placeholder?: string;
  hint?: string;
  label?: string;
}) {
  const box = useRef<HTMLTextAreaElement>(null);
  const panel = useRef<HTMLDivElement>(null);
  const [text, setText] = useState("");
  useEffect(() => {
    box.current?.focus();
    const down = (e: MouseEvent) => !panel.current?.contains(e.target as Node) && onClose();
    document.addEventListener("mousedown", down);
    return () => document.removeEventListener("mousedown", down);
  }, [onClose]);
  const ask = () => text.trim() && onAsk(text.trim());
  const sendKey = useSendKey();
  return (
    <PagePanel ref={panel} at={{ top, left: 0 }} label={label}>
      <PageComposer field={box} value={text} onChange={setText} onSend={ask} tallest={160} placeholder={placeholder} aria-label={label} escape={onClose}>
        <Button variant="primary" size="small" disabled={!text.trim()} onClick={ask} title="Send" keys={sendKeys(sendKey)}>
          Ask <SendKeys />
        </Button>
      </PageComposer>
      <PagePanelFoot hint={hint} />
    </PagePanel>
  );
}
