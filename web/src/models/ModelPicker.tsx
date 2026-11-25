import { useMemo, useRef, useState } from "react";
import type { ModelInfo } from "../api";
import { Button } from "../ui/Button";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { HeldPopover } from "../ui/Popover";
import { rowOf, shortOf, treeOf } from "./modelTree";
import css from "./ModelPicker.module.css";
import { ModelPanel } from "./ModelPanel";

// ModelPicker is a model picked from what a harness lists, as a role's
// is in Settings: a button that says what's set, by its model and what it
// chooses of it ("Claude Opus 5.5 · high · fast"), and opens the panel to
// find another (ModelPanel). What's set that the harness doesn't list is
// said as it's written; nothing set is placeholder. onAnother is for a
// name typed instead; open starts it open, for a picture of it.
export function ModelPicker({
  models,
  value,
  placeholder,
  busy,
  stretch,
  open: opened,
  onChange,
  onAnother,
  "aria-label": label,
}: {
  models: ModelInfo[];
  value: string;
  placeholder: string;
  busy?: boolean;
  stretch?: boolean;
  open?: boolean;
  onChange: (id: string) => void;
  onAnother?: () => void;
  "aria-label": string;
}) {
  const [open, setOpen] = useState(!!opened);
  const at = useRef<HTMLButtonElement>(null);
  const tree = useMemo(() => treeOf(models), [models]);
  const row = value ? rowOf(tree, value) : undefined;
  const set = row?.models.find((m) => m.id === value);
  const says = row && set ? shortOf(row, set) : value;
  return (
    <>
      <Button ref={at} className={cx(css.button, stretch && css.stretch)} onClick={() => setOpen(!open)} aria-label={`${label}: ${says || placeholder}`} data-field="">
        <span className={cx(css.value, !says && css.placeholder)}>{says || placeholder}</span>
        <Icon name="chevron-down" size={14} className={css.chevron} />
      </Button>
      <HeldPopover open={open} onOpenChange={setOpen} at={at} aria-label={label} placement="bottom start" maxHeight={600}>
        <ModelPanel
          models={models}
          value={value}
          busy={busy}
          onUse={(id) => {
            setOpen(false);
            if (id !== value) onChange(id);
          }}
          onAnother={
            onAnother &&
            (() => {
              setOpen(false);
              onAnother();
            })
          }
        />
      </HeldPopover>
    </>
  );
}
