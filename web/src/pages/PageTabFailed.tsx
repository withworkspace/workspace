import { Icon } from "../ui/Icon";
import css from "./PageTabFailed.module.css";

// PageTabFailed is on a pass's tab when its last run couldn't finish.
export function PageTabFailed() {
  return <Icon name="alert" size={11} className={css.failed} />;
}
