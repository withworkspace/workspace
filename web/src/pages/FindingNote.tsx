import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { EditorView } from "prosemirror-view";
import { type DismissWhy, dismissWhys, type PagePass, type PassFinding } from "../api";
import { noteElement } from "../editor/notes";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { FindingBody } from "./FindingBody";
import { PageDismissAs } from "./PageDismissAs";
import { PageFinding } from "./PageFinding";
import { PageFindingActs } from "./PageFindingActs";
import { PageNoteFrom } from "./PageNoteFrom";
import { PagePanel } from "./PagePanel";

// FindingNote is a pass's finding, opened from its words on the page:
// under them, with what to do about it.
export function FindingNote({
  note,
  pass,
  finding,
  view,
  wrap,
  onClose,
  onSettle,
  onTake,
  onFindOut,
  onPass,
}: {
  note: string;
  pass: PagePass;
  finding: PassFinding;
  view: EditorView;
  wrap: React.RefObject<HTMLDivElement | null>;
  onClose: () => void;
  onSettle: (state: PassFinding["state"], why?: DismissWhy) => void;
  onTake: () => void;
  onFindOut: () => void;
  onPass: () => void;
}) {
  const panel = useRef<HTMLDivElement>(null);
  const [at, setAt] = useState<{ top: number; left: number }>();
  useLayoutEffect(() => {
    const mark = noteElement(view, note);
    const host = wrap.current;
    if (!mark || !host) return onClose();
    const r = mark.getBoundingClientRect();
    const w = host.getBoundingClientRect();
    setAt({ top: r.bottom - w.top + 6, left: Math.max(0, Math.min(r.left - w.left, w.width - (panel.current?.offsetWidth ?? 440))) });
  }, [view, note, wrap, onClose]);
  useEffect(() => {
    const down = (e: MouseEvent) => {
      const t = e.target as Element;
      if (panel.current?.contains(t) || t.closest?.("[data-note]")) return;
      onClose();
    };
    const key = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    document.addEventListener("mousedown", down);
    document.addEventListener("keydown", key);
    return () => {
      document.removeEventListener("mousedown", down);
      document.removeEventListener("keydown", key);
    };
  }, [onClose]);
  return (
    <PagePanel ref={panel} at={at} note label={`From ${pass.title}`}>
      <PageNoteFrom pass={pass.title} onPass={onPass} onClose={onClose} />
      <PageFinding look="in">
        <FindingBody f={finding} />
        <PageFindingActs>
          {finding.suggest && (
            <Button variant="primary" size="small" onClick={onTake}>
              Use these words
            </Button>
          )}
          <Button size="small" onClick={() => onSettle("done")}>
            <Icon name="check" size={12} /> Done
          </Button>
          <Button variant="ghost" size="small" onClick={onFindOut}>
            Find out
          </Button>
          <PageDismissAs whys={dismissWhys} onDismiss={(why) => onSettle("dismissed", why)} />
        </PageFindingActs>
      </PageFinding>
    </PagePanel>
  );
}
