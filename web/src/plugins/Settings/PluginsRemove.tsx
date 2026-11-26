import { useState } from "react";
import { Button } from "../../ui/Button";
import { Checkbox } from "../../ui/Checkbox";
import { Dialog, DialogActions, DialogText } from "../../ui/Dialog";
import css from "./PluginsRemove.module.css";
import { Text } from "../../ui/Text";

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
        Stops the plugin and removes its sidebar view and commands
        {names?.length ? `, and its refs (${names.join(", ")}) stop being links` : ""}. Config in{" "}
        <code>ws.toml</code> and <code>ws.local.toml</code> stays: remove it yourself.
      </DialogText>
      <Checkbox className={css.data} checked={data} onChange={setData}>
        <span>
          Also remove data and secrets
          <Text dim className={css.alone}> If kept, adding it again picks up where it left off.</Text>
        </span>
      </Checkbox>
      <DialogActions>
        <Button variant="ghost" onClick={onClose} keys="esc">
          Cancel
        </Button>
        <Button variant="danger" onClick={() => onRemove(data)}>
          {data ? "Remove with data" : "Remove"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
