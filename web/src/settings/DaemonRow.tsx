import { askShutDown } from "../shell/daemon";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Marked } from "./Marked";
import { words } from "./sections";
import { SettingsRow } from "./SettingsRow";

// DaemonRow shuts the daemon down, asking first: the workspace's menu
// has it too.
export function DaemonRow() {
  return (
    <SettingsRow label={<Marked text={words.daemon.label} />} about={<Marked text={words.daemon.about} />}>
      <Button variant="danger" type="button" onClick={() => void askShutDown()}>
        <Icon name="power" size={13} />
        Shut down…
      </Button>
    </SettingsRow>
  );
}
