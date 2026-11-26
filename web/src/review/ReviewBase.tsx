import { Icon } from "../ui/Icon";
import { Text } from "../ui/Text";
import css from "./ReviewBase.module.css";

// ReviewBase is what a change goes into, in its bar: said only when it
// isn't the repo's default branch, which is what's taken as read.
export function ReviewBase({ base }: { base: string }) {
  return (
    <span className={css.base} title={`Goes into ${base}`}>
      <Icon name="branch" size={12} /> into <Text mono>{base}</Text>
    </span>
  );
}
