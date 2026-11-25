import { useKeys } from "../keys";
import { globalKeys } from "../shell/helpKeys";
import { Dialog } from "../ui/Dialog";
import { WindowKeys } from "./WindowKeys";

// KeysHelp is the Keys dialog: the keys of the page that's open (keys),
// then those everywhere. ? closes it again.
export function KeysHelp({ keys, onClose }: { keys: [string, string][]; onClose: () => void }) {
  useKeys({ "?": onClose }, { modal: true });
  return (
    <Dialog title="Keys" onClose={onClose} wide>
      <WindowKeys groups={[...(keys.length > 0 ? [{ title: "On this page", keys }] : []), { title: "Everywhere", keys: globalKeys }]} />
    </Dialog>
  );
}
