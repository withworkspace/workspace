import { Button } from "../../ui/Button";
import { Chip } from "../../ui/Chip";
import { Icon } from "../../ui/Icon";
import { Spinner } from "../../ui/Spinner";
import { Time } from "../../ui/Time";
import css from "./PluginsUpdates.module.css";
import { Text } from "../../ui/Text";

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
      <Text dim className={css.looked}>
        {checked ? (
          <>
            Checked for updates <Time iso={checked} as="relative" />
          </>
        ) : (
          "Not checked for updates yet"
        )}
      </Text>
      <Button size="small" onClick={onLook} disabled={looking} title="Check each plugin's repo now">
        {looking ? <Spinner size={12} /> : <Icon name="rotate" size={12} />} {looking ? "Checking…" : "Check for updates"}
      </Button>
    </div>
  );
}
