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
          placeholder="owner/repo, or a folder on this machine"
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
        A repo on GitHub is installed from its latest release (<code>owner/repo@v1.2.0</code> for another), checked against the release's checksums; one with no build for
        this machine is built here from its source. A folder is copied in, by its whole path or from <code>~</code>. A plugin is a program that runs as you: add one as
        you'd install any other.
      </p>
    </form>
  );
}
