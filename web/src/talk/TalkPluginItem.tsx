import { Markdown } from "../markdown/LazyMarkdown";
import { Checkbox } from "../ui/Checkbox";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/Inline";
import css from "./TalkPluginItem.module.css";
import { Text, textClass } from "../ui/Text";
import { Prose } from "../ui/Prose";

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
              <Text dim className={css.n}>{n}</Text>
            </Checkbox>
          ) : (
            <Text dim className={css.n}>{n}</Text>
          )}
          {title !== undefined && <span className={css.title}>{title}</span>}
          {on && (
            <Text dim className={css.on}>
              on <Inline text={on} />
            </Text>
          )}
          <span className={css.spacer} />
          {made && (
            <span className={css.made}>
              <Icon name="check" size={12} /> {made !== true ? <Inline text={made} /> : "Done"}
            </span>
          )}
          {left && <Text dim>Not accepted</Text>}
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
                    <del className={textClass("dim")}>{r.was || "none"}</del> →{" "}
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
            <summary className={textClass("dim")}>Description</summary>
            <Prose>
              <Markdown text={markdown} />
            </Prose>
          </details>
        ) : (
          <Prose className={css.text}>
            <Markdown text={markdown} />
          </Prose>
        ))}
      {problem && (
        <div className={css.problem}>
          <Icon name="alert" size={13} /> Not done: {problem}
        </div>
      )}
      {say && <Text as="div" dim className={css.say}>{say}</Text>}
    </li>
  );
}
