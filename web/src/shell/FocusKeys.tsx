import { useLocation } from "react-router";
import { useCommands } from "../keymap/useCommands";
import { everywhereKeys } from "./keys";
import { toast } from "../ui/toast";
import { usePlugins, useWorkspaceIfRead } from "../workspace/store";
import { pageRefAt, pinNameOf } from "./pinNames";
import { useFocus } from "./useFocus";
import { useSidebarKnown } from "./useSidebarKnown";

// FocusKeys is . anywhere: pin what the page is, or unpin it.
export function FocusKeys() {
  const workspace = useWorkspaceIfRead();
  const plugins = usePlugins();
  const focus = useFocus();
  const { pathname, search } = useLocation();
  const known = useSidebarKnown(workspace, plugins);
  useCommands(everywhereKeys, {
    pin: () => {
      const r = focus?.supported ? pageRefAt(pathname, search, known) : undefined;
      const name = r && pinNameOf(r);
      if (!name || !focus) return;
      if (focus.pinned(name)) focus.unpin(name);
      else {
        focus.pin(name);
        toast(`Pinned ${name}`);
      }
    },
  });
  return null;
}
