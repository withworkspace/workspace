import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { effortWords, type ModelRow } from "./modelTree";
import css from "./ModelRow.module.css";

// ModelRow is one model in a model picker's list: its name (and
// what the harness says of it, where it's one name alone), the efforts it
// runs at, whether it thinks and has a fast way, and whether
// what's set is one of its names (chosen) or the agent's default is. on
// is the picker's cursor being on it; the focus stays in the picker's
// text box, which says so (id).
export function ModelRow({ id, row, on, chosen, onPress, onUse }: { id: string; row: ModelRow; on: boolean; chosen: boolean; onPress: () => void; onUse: () => void }) {
  const named = row.efforts.filter(Boolean);
  const range = named.length > 1 ? `${effortWords[named[0]]}–${effortWords[named[named.length - 1]]}` : named.length ? effortWords[named[0]] : "";
  return (
    <li id={id} role="option" aria-selected={on} className={cx(css.row, chosen && css.chosen)} onClick={onPress} onDoubleClick={onUse}>
      <span className={css.label}>{row.label}</span>
      {row.models.length === 1 && row.models[0].about && <span className={css.about}>{row.models[0].about}</span>}
      {row.default && <span className={css.note}>default</span>}
      <span className={css.has}>
        {range && <span className={css.efforts}>{range}</span>}
        {row.thinking.includes(true) && <Icon name="sparkle" size={13} className={css.mark} title="Thinks" />}
        {row.fast.includes(true) && <Icon name="bolt" size={13} className={css.mark} title="Has a fast way" />}
      </span>
      <span className={css.tick}>{chosen && <Icon name="check" size={14} />}</span>
    </li>
  );
}
