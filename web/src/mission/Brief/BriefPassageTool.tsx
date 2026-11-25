import { type RefObject, useRef } from "react";
import { SelectionToolbar } from "../../ask/SelectionToolbar";
import { usePassage } from "./usePassage";

// BriefPassageTool is what's offered of a passage picked out of the
// documents: asking the planner about it. It goes as the documents scroll
// under it.
export function BriefPassageTool({ within, ask }: { within: RefObject<HTMLElement | null>; ask: (passage: string) => void }) {
  const tool = useRef<HTMLDivElement>(null);
  const [at, gone] = usePassage(within, tool);
  if (!at) return null;
  return (
    <SelectionToolbar
      ref={tool}
      left={at.left}
      top={at.top}
      askTitle="Ask the planner about this passage"
      onAsk={() => {
        gone();
        ask(at.passage);
        getSelection()?.removeAllRanges();
      }}
    />
  );
}
