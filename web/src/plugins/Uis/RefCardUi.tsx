import { RefCard } from "../../refs/RefCard";
import { refOf } from "../../refs/refs";
import { useWorkspaceIfRead } from "../../workspace/store";
import type { UiProps } from "./uiProps";

// RefCardUi is a ref's card, by the ref's words in the app's page: the
// card ws shows for a ref in its own pages.
export function RefCardUi({ asked, anchor, stay, leave }: UiProps) {
  const workspace = useWorkspaceIfRead();
  const r = typeof asked.with.ref === "string" ? refOf(asked.with.ref) : undefined;
  // A ref ws can't read has no card.
  if (!r || !anchor || !workspace) return null;
  return <RefCard r={r} el={anchor} onEnter={stay} onLeave={leave} hint={false} />;
}
