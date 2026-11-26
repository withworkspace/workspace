import { Icon } from "../../ui/Icon";
import css from "./FilesCapsule.module.css";
import { Text, textClass } from "../../ui/Text";

// FilesCapsule is which capsule a mission's explorer is of, on its bar:
// its repo and its name.
export function FilesCapsule({ repo, name }: { repo: string; name: string }) {
  return (
    <div className={css.capsule}>
      <Icon name="box" size={14} className={textClass("dim")} />
      <Text mono className={css.name}>
        {repo}/{name}
      </Text>
    </div>
  );
}
