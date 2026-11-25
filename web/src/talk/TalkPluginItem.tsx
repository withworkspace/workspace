import { Markdown } from "../markdown/LazyMarkdown";
import { Checkbox } from "../ui/Checkbox";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/Inline";
import css from "./TalkPluginItem.module.css";

// TalkPluginItem is an item of what a plugin proposed: its number (n),
// ticked to accept (pick) while there's a choice; its title, unless it's
// the card's; the thing it's on; once done, what it made (made: true when
// the plugin didn't name it); left, not accepted beside what was. Then
// its rows (what was, for a change), what it says (folded when long), why
// it wasn't done (problem), and what the plugin said of it once done
// (say). alone: it's the card's only item.
export function TalkPluginItem({
  n,
  title,
  on,
  made,
  left,
  pick,
  rows,
  markdown,
  long,
  problem,
  say,
  alone,
}: {
  n: number;
  title?: string;
  on?: string;
  made?: string | true;
  left?: boolean;
  pick?: { picked: boolean; onPick: () => void };
  rows?: { label: string; was?: string; value: string }[];
  markdown?: string;
  long?: boolean;
  problem?: string;
  say?: string;
  alone?: boolean;
}) {
  return (
    <li className={cx(css.item, alone && css.alone, left && css.left)}>
      {(title !== undefined || pick || on || made || left) && (
        <div className={css.head}>
          {pick ? (
            <Checkbox className={css.pick} checked={pick.picked} onChange={pick.onPick} aria-label={`Accept item ${n}`}>
              <span className={cx(css.n, "dim")}>{n}</span>
            </Checkbox>
          ) : (
            <span className={cx(css.n, "dim")}>{n}</span>
          )}
          {title !== undefined && <span className={css.title}>{title}</span>}
          {on && (
            <span className={cx(css.on, "dim")}>
              on <Inline text={on} />
            </span>
          )}
          <span className={css.spacer} />
          {made && (
            <span className={css.made}>
              <Icon name="check" size={12} /> {made !== true ? <Inline text={made} /> : "Done"}
            </span>
          )}
          {left && <span className="dim">Not accepted</span>}
        </div>
      )}
      {!!rows?.length && (
        <dl className={css.rows}>
          {rows.map((r, i) => (
            <div key={i}>
              <dt>{r.label}</dt>
              <dd>
                {r.was !== undefined && (
                  <>
                    <del className="dim">{r.was || "none"}</del> →{" "}
                  </>
                )}
                <Inline text={r.value} />
              </dd>
            </div>
          ))}
        </dl>
      )}
      {markdown?.trim() &&
        (long ? (
          <details className={css.text}>
            <summary className="dim">What it says</summary>
            <div className="prose">
              <Markdown text={markdown} />
            </div>
          </details>
        ) : (
          <div className={cx(css.text, "prose")}>
            <Markdown text={markdown} />
          </div>
        ))}
      {problem && (
        <div className={css.problem}>
          <Icon name="alert" size={13} /> Not done: {problem}
        </div>
      )}
      {say && <div className={cx("dim", css.say)}>{say}</div>}
    </li>
  );
}
