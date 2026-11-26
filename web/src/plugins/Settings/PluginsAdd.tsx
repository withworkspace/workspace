import { Button } from "../../ui/Button";
import { Icon } from "../../ui/Icon";
import { Input } from "../../ui/Input";
import { Spinner } from "../../ui/Spinner";
import css from "./PluginsAdd.module.css";

type PluginsAddProps = {
  // What's typed: the repo a plugin's published at, or a folder.
  value: string;
  onChange: (value: string) => void;
  // It's being added.
  busy: boolean;
  onAdd: () => void;
};

// PluginsAdd is the field a plugin's added from: the repo it's published
// at, or a folder on this machine, with what adding one does under it.
export function PluginsAdd({ value, onChange, busy, onAdd }: PluginsAddProps) {
  return (
    <form
      className={css.add}
      onSubmit={(e) => {
        e.preventDefault();
        onAdd();
      }}
    >
      <label className={css.field}>
        <span className={css.label}>Add a plugin</span>
        <Input
          type="text"
          value={value}
          placeholder="owner/repo or local folder"
          spellCheck={false}
          autoComplete="off"
          autoCapitalize="off"
          disabled={busy}
          onChange={(e) => onChange(e.target.value)}
          aria-describedby="plugins-add-about"
        />
      </label>
      <Button variant="primary" type="submit" disabled={!value.trim() || busy}>
        {busy ? <Spinner size={12} /> : <Icon name="plus" size={12} />} {busy ? "Adding…" : "Add"}
      </Button>
      <p id="plugins-add-about" className={css.about}>
        GitHub repos install from their latest release (<code>owner/repo@v1.2.0</code> for another), verified
        by checksum, or build from source. Folders, by full path or from <code>~</code>, are copied in.
        Plugins run as you: only add ones you trust.
      </p>
    </form>
  );
}
