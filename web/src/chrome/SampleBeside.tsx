import { useState } from "react";
import { Composer } from "../panes/Composer";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Dock } from "./Dock";
import { DockBody } from "./DockBody";
import { DockDivide } from "./DockDivide";
import { DockEdge } from "./DockEdge";
import { DockHead } from "./DockHead";
import { DockTab } from "./DockTab";
import { DockTabList } from "./DockTabList";
import { DockTabs } from "./DockTabs";
import type { SampleThing } from "./dockSamples";
import { PaneHead } from "./PaneHead";

type SampleBesideProps = {
  // What's open in it to start with, and what's the page's own before
  // those: tabs with no way to close them.
  things: SampleThing[];
  own?: SampleThing[];
  // Over the page or next to it (which is switched at the foot).
  over: boolean;
  size: number;
  onSize: (size: number) => void;
  // Closing it: a sheet's button, or its last tab closed.
  onClose: () => void;
  // On a narrow window: a sheet, with no edge and no floating.
  sheet?: boolean;
  // The one of them that's a peek, by its id.
  peek?: string;
};

// SampleBeside is Beside as the stories draw it: the dock's parts put
// together as the app will, over hand-written things (dockSamples.tsx).
// Which tabs are open and which is in front is kept here; whether it's
// open, over the page, and how wide, is the window's, so the story has
// them.
export function SampleBeside({ things, own = [], over, size, onSize, onClose, sheet, peek }: SampleBesideProps) {
  const [open, setOpen] = useState(things);
  const [front, setFront] = useState((things[0] ?? own[0])?.id);
  const all = [...own, ...open];
  const now = all.find((t) => t.id === front) ?? all[0];
  if (!now) return null;
  const close = (id: string) => {
    const left = open.filter((t) => t.id !== id);
    setOpen(left);
    if (left.length + own.length === 0) return onClose();
    if (front === id) setFront((left[open.findIndex((t) => t.id === id)] ?? left[left.length - 1] ?? own[0]).id);
  };
  const tab = (t: SampleThing, mine: boolean) => <DockTab key={t.id} name={t.name} lead={t.lead} on={t.id === now.id} passing={t.id === peek ? "a peek, which the next takes the place of" : undefined} onPick={() => setFront(t.id)} onClose={mine ? () => close(t.id) : undefined} />;
  const listed = (t: SampleThing) => ({ id: t.id, name: t.name, lead: t.lead, on: t.id === now.id, onPick: () => setFront(t.id) });
  return (
    <Dock side="beside" label="Beside" size={size} over={over && !sheet} sheet={sheet} edge={<DockEdge side="beside" label="Beside's width" size={size} min={300} max={760} rest={420} onSize={onSize} />}>
      <DockHead
        tools={
          sheet && (
            <Button variant="ghost" size="small" square title="Close Beside" onClick={onClose}>
              <Icon name="x" size={13} />
            </Button>
          )
        }
      >
        <DockTabs
          label="Open beside"
          list={
            <DockTabList
              lots={[
                { name: "The page's", tabs: own.map(listed) },
                { name: "Yours", tabs: open.map(listed) },
              ]}
            />
          }
        >
          {own.map((t) => tab(t, false))}
          {own.length > 0 && open.length > 0 && <DockDivide />}
          {open.map((t) => tab(t, true))}
        </DockTabs>
      </DockHead>
      <DockBody key={now.id} label={now.name} pad>
        <PaneHead to="/" sub={now.sub} />
        {now.body}
      </DockBody>
      {now.says && <Composer key={`say:${now.id}`} placeholder={now.says} onSay={() => new Promise((said) => setTimeout(said, 400))} />}
    </Dock>
  );
}
