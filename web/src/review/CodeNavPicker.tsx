import { useState } from "react";
import { useCommands } from "../keymap/useCommands";
import { reviewPickKeys } from "./keys";
import { CodeNavList } from "./CodeNavList";
import css from "./CodeNavPicker.module.css";
import { Text } from "../ui/Text";

// CodeNavPicker is d's list: the names on a line (rect, the line's), to
// follow one (j, k, ⏎ or d). While it's open the keys are its own.
export function CodeNavPicker({ names, rect, onClose, onPick }: { names: { from: number; text: string; part: string }[]; rect: DOMRect; onClose: () => void; onPick: (i: number) => void }) {
  const [pick, setPick] = useState(0);
  const n = names.length;
  useCommands(
    reviewPickKeys,
    {
      close: onClose,
      next: () => setPick((i) => Math.min(i + 1, n - 1)),
      prev: () => setPick((i) => Math.max(i - 1, 0)),
      go: () => onPick(pick),
      follow: () => onPick(pick),
    },
    { modal: true },
  );
  const top = Math.min(rect.bottom + 4, window.innerHeight - 40 - n * 26);
  return (
    <div className={css.picker} style={{ top, left: Math.max(8, rect.left + 120) }} onMouseDown={(e) => e.stopPropagation()}>
      <Text as="div" dim className={css.how}>Go to the definition of</Text>
      <CodeNavList
        inline
        rows={names.map((x, i) => ({
          key: `${x.from}`,
          picked: i === pick,
          onEnter: () => setPick(i),
          onClick: () => onPick(i),
          children: (
            <>
              <code>{x.text}</code>
              <Text dim>{x.part === "m" ? "member" : x.part === "d" ? "defined here" : x.part === "i" ? "import" : ""}</Text>
            </>
          ),
        }))}
      />
    </div>
  );
}
