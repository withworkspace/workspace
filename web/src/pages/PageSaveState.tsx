import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import css from "./PageSaveState.module.css";
import { PageRetry } from "./PageRetry";

export type Saving = "saved" | "unsaved" | "saving" | "changed" | "failed";

// PageSaveState says where saving the page stands: saved, on its way, not
// saving (it changed somewhere else), or not saved, to try again.
export function PageSaveState({ state, retry }: { state: Saving; retry: () => void }) {
  if (state === "failed") return <PageRetry onClick={retry}>Not saved: try again</PageRetry>;
  const words = { saved: "Saved", unsaved: "Saving…", saving: "Saving…", changed: "Not saving" }[state];
  return (
    <span className={cx(css.state, state === "changed" && css.changed)} role="status">
      {state === "saved" && <Icon name="check" size={12} />}
      {words}
    </span>
  );
}
