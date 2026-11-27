import { useState } from "react";
import { helpGroups } from "../keymap/help";
import { useCommands } from "../keymap/useCommands";
import { everywhereKeys } from "../shell/keys";
import { Dialog } from "../ui/Dialog";
import { WindowKeys } from "./WindowKeys";

// KeysHelp is the Keys dialog: the keys of what's open (a group for the
// page's, and each part in it that has keys), then those everywhere, as
// they are now (yours, where you've set them: Settings › Keyboard). Its
// own key closes it again.
export function KeysHelp({ onClose }: { onClose: () => void }) {
  useCommands(everywhereKeys, { help: onClose }, { modal: true });
  const [groups] = useState(() => helpGroups(everywhereKeys.help.scope));
  return (
    <Dialog title="Keys" onClose={onClose} wide>
      <WindowKeys groups={groups} />
    </Dialog>
  );
}
