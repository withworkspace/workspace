import { useMemo, useRef, useState } from "react";
import type { ModelInfo } from "../api";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { HeldPopover } from "../ui/Popover";
import { Spinner } from "../ui/Spinner";
import { rowOf, shortOf, treeOf } from "./modelTree";
import { ModelPanel } from "./ModelPanel";
import css from "./ModelSwitch.module.css";

// ModelSwitch is the model something running is on, and switching it: a
// quiet button in its head (a chip's icon, then the model as the picker
// says it: "Claude Opus 5.5 · high"), which opens the model panel over
// what its harness lists (models). What's on that the harness doesn't
// list is said as it's written; nothing is placeholder. A model used is
// onSwitch's to do; busy is it doing it. unavailable is why it can't be
// switched now, said as a Button says it.
export function ModelSwitch({
  models,
  value,
  placeholder,
  busy,
  asking,
  unavailable,
  onSwitch,
  "aria-label": label,
}: {
  models: ModelInfo[];
  value: string;
  placeholder: string;
  busy?: boolean;
  asking?: boolean;
  unavailable?: string;
  onSwitch: (id: string) => void;
  "aria-label": string;
}) {
  const [open, setOpen] = useState(false);
  const at = useRef<HTMLButtonElement>(null);
  const tree = useMemo(() => treeOf(models), [models]);
  const row = value ? rowOf(tree, value) : undefined;
  const set = row?.models.find((m) => m.id === value);
  const says = (row && set ? shortOf(row, set) : value) || placeholder;
  return (
    <>
      <Button ref={at} variant="ghost" size="small" className={css.switch} onClick={() => setOpen(!open)} unavailable={unavailable} title={value || undefined} aria-label={`${label}: ${says}`}>
        {busy ? <Spinner size={12} /> : <Icon name="cpu" size={13} className={css.icon} />}
        <span className={css.value}>{says}</span>
        <Icon name="chevron-down" size={12} className={css.chevron} />
      </Button>
      <HeldPopover open={open} onOpenChange={setOpen} at={at} aria-label={label} placement="bottom start" maxHeight={600}>
        <ModelPanel
          models={models}
          value={value}
          busy={asking}
          onUse={(id) => {
            setOpen(false);
            if (id !== value) onSwitch(id);
          }}
        />
      </HeldPopover>
    </>
  );
}
