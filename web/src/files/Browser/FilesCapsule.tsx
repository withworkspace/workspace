import { Icon } from "../../ui/Icon";
import { cx } from "../../ui/cx";
import css from "./FilesCapsule.module.css";

// FilesCapsule is which capsule a mission's explorer is of, on its bar:
// its repo and its name.
export function FilesCapsule({ repo, name }: { repo: string; name: string }) {
  return (
    <div className={css.capsule}>
      <Icon name="box" size={14} className="dim" />
      <span className={cx("mono", css.name)}>
        {repo}/{name}
      </span>
    </div>
  );
}
