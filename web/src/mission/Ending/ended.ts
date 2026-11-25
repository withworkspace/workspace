import { act } from "../../api";
import type { Reload } from "../../load";
import { humanError } from "../../ui/text";
import { toast } from "../../ui/toast";

// ended toasts an ending, with an undo, and reads the mission again: its
// promise is that read's (Reload).
export function ended(slug: string, message: string, reload: Reload): Promise<void> {
  toast(message, {
    tone: "done",
    action: {
      label: "Undo",
      run: () =>
        act(slug, "unend").then(
          (r) => {
            toast(r.message);
            reload();
          },
          (e: Error) => toast(humanError(e.message), { tone: "danger" }),
        ),
    },
  });
  return reload();
}
