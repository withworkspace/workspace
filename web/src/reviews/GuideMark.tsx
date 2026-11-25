import type { ReviewGuide } from "../api";
import { Icon } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import css from "./GuideMark.module.css";

// GuideMark is how the guide stands, beside its name in the list down the
// side: being written, couldn't be written, or written before the last
// push.
export function GuideMark({ guide }: { guide?: ReviewGuide }) {
  if (guide?.running) return <Spinner size={11} title="Writing the guide…" />;
  if (guide?.failed) return <Icon name="alert" size={11} className={css.bad} title="The guide couldn't be written" />;
  if (guide?.guide && guide.moved) return <Icon name="rotate" size={11} className="dim" title="It's been pushed to since the guide was written" />;
  return null;
}
