import { useState } from "react";
import { Button } from "../../ui/Button";
import { Checkbox } from "../../ui/Checkbox";
import { cx } from "../../ui/cx";
import { Dialog, DialogActions, DialogText } from "../../ui/Dialog";
import css from "./PluginsRemove.module.css";

type PluginsRemoveProps = {
  name: string;
  // How its things are named, which are words again once it's gone.
  names?: string[];
  onClose: () => void;
  onRemove: (data: boolean) => void;
};

// PluginsRemove asks before a plugin's removed, and whether what it kept
// for itself goes with it.
export function PluginsRemove({ name, names, onClose, onRemove }: PluginsRemoveProps) {
  const [data, setData] = useState(false);
  return (
    <Dialog title={`Remove ${name}?`} onClose={onClose}>
      <DialogText>
        It's stopped and taken out of the workspace: its view leaves the sidebar, its commands go
        {names?.length ? `, and its things' names (${names.join(", ")}) are words again wherever they're written` : ""}. Its config in <code>ws.toml</code> and{" "}
        <code>ws.local.toml</code> is yours to remove.
      </DialogText>
      <Checkbox className={css.data} checked={data} onChange={setData}>
        <span>
          Remove what it kept for itself too: its data, and its secrets
          <span className={cx("dim", css.alone)}> Left alone, adding it again picks up where it was.</span>
        </span>
      </Checkbox>
      <DialogActions>
        <Button variant="ghost" onClick={onClose}>
          Cancel
        </Button>
        <Button variant="danger" onClick={() => onRemove(data)}>
          {data ? "Remove, and what it kept" : "Remove"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
