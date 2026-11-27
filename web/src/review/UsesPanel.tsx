import { useEffect, useRef, useState } from "react";
import { useCommand, useCommands } from "../keymap/useCommands";
import { reviewPanelKeys } from "./keys";
import { CodeNavPanel } from "./CodeNavPanel";
import { CodeNavUses } from "./CodeNavUses";
import { groupUses, type NavRef, type NavReferences, type Side } from "./nav";

// UsesPanel is who uses a definition: in this change first, then
// elsewhere, maybes folded under each. Its keys are its own while it's
// open: j and k move down the uses (maybes only once they're shown), ⏎
// goes to one.
export function UsesPanel(props: {
  at: { side: Side };
  uses: NavReferences;
  inChange: (path: string) => boolean;
  changed: (path: string, line: number) => boolean;
  onGo: (u: NavRef) => void;
  sideWords: (side: Side) => string;
  onClose: () => void;
  onBack: () => void;
}) {
  const { at, uses, inChange, changed, onGo, sideWords, onClose, onBack } = props;
  const groups = groupUses(uses.uses, inChange, changed);
  const [open, setOpen] = useState<Record<string, boolean>>({});
  // The list as it's moved through: maybes only when opened.
  const flat = groups.flatMap((g) => [...g.uses, ...(open[g.title] ? g.maybes : [])]);
  const [pick, setPick] = useState(0);
  const close = useCommand(reviewPanelKeys.close, onClose, { modal: true });
  const goBack = useCommand(reviewPanelKeys.back, onBack, { modal: true });
  useCommands(
    reviewPanelKeys,
    {
      down: () => setPick((i) => Math.min(i + 1, flat.length - 1)),
      up: () => setPick((i) => Math.max(i - 1, 0)),
      go: () => flat[pick] && onGo(flat[pick]),
    },
    { modal: true },
  );
  const body = useRef<HTMLDivElement>(null);
  useEffect(() => {
    body.current?.querySelector("[data-picked]")?.scrollIntoView({ block: "nearest" });
  }, [pick]);
  const use = (u: NavRef) => {
    const i = flat.indexOf(u);
    return { key: `${u.path}:${u.line}:${u.col}`, path: u.path, line: u.line, text: u.text, changed: changed(u.path, u.line), picked: i === pick, onEnter: () => i >= 0 && setPick(i), onGo: () => onGo(u) };
  };
  const count = uses.uses.filter((u) => !u.maybe).length;
  return (
    <CodeNavPanel
      label={`Who uses ${uses.name}`}
      title={
        <span>
          Who uses <code>{uses.name}</code>
        </span>
      }
      said={
        <>
          {count} {count === 1 ? "use" : "uses"}
          {uses.partial ? ", maybe more" : ""} · {sideWords(at.side)}
        </>
      }
      uses
      body={body}
      onBack={goBack}
      onClose={close}
    >
      <CodeNavUses
        def={uses.def ? { signature: uses.def.signature || uses.name, path: uses.def.path, line: uses.def.line } : undefined}
        groups={groups.map((g) => ({ title: g.title, named: groups.length > 1 || g.title !== "Elsewhere", uses: g.uses.map(use), maybes: g.maybes.map(use), open: !!open[g.title] }))}
        onMaybes={(t) => setOpen((o) => ({ ...o, [t]: !o[t] }))}
      />
    </CodeNavPanel>
  );
}
