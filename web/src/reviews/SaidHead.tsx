import type { Said } from "../api";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Time } from "../ui/Time";
import { saidIcon, saidWords } from "./saidWords";
import css from "./SaidHead.module.css";
import { Text, textClass } from "../ui/Text";

// SaidHead is who said something on a pull request, what they did (in its
// colour: approved green, changes red), whether it's of earlier commits,
// when (or that it's on its way), and a link to it on GitHub.
export function SaidHead({ it }: { it: Said }) {
  return (
    <>
      <strong>{it.mine ? "You" : it.by || "Someone"}</strong>
      <span className={cx(css.what, it.kind === "approved" && css.approved, it.kind === "changes" && css.changes)}>
        <Icon name={saidIcon(it.kind)} size={12} /> {saidWords[it.kind]}
      </span>
      {it.stale && (
        <Text dim title="New commits since this review">
          on earlier commits
        </Text>
      )}
      {it.sending ? (
        <Text dim title="Sending to GitHub">
          Sending…
        </Text>
      ) : (
        <Time iso={it.at} as="relative" className={textClass("dim")} />
      )}
      {it.url && (
        <a className={textClass("link", "dim")} href={it.url} target="_blank" rel="noreferrer" title="See it on GitHub" onClick={(e) => e.stopPropagation()}>
          <Icon name="external" size={11} />
        </a>
      )}
    </>
  );
}
