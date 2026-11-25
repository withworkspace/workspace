import { Button } from "../../ui/Button";
import { Chip } from "../../ui/Chip";
import { cx } from "../../ui/cx";
import { Icon } from "../../ui/Icon";
import { Spinner } from "../../ui/Spinner";
import { Time } from "../../ui/Time";
import css from "./PluginsUpdates.module.css";

type PluginsUpdatesProps = {
  // How many have a later release out.
  out: number;
  // When the daemon last looked, if it has.
  checked?: string;
  // It's looking now.
  looking: boolean;
  onLook: () => void;
};

// PluginsUpdates is what's out for the plugins from a repo, when that
// was looked for, and the way to look now.
export function PluginsUpdates({ out, checked, looking, onLook }: PluginsUpdatesProps) {
  return (
    <div className={css.updates}>
      {out > 0 && (
        <Chip tone="attention">
          {out} {out === 1 ? "update" : "updates"}
        </Chip>
      )}
      <span className={cx("dim", css.looked)}>
        {checked ? (
          <>
            Looked for updates <Time iso={checked} as="relative" />
          </>
        ) : (
          "Not looked for updates yet"
        )}
      </span>
      <Button size="small" onClick={onLook} disabled={looking} title="Ask each plugin's repo for its latest release now">
        {looking ? <Spinner size={12} /> : <Icon name="rotate" size={12} />} {looking ? "Looking…" : "Check for updates"}
      </Button>
    </div>
  );
}
