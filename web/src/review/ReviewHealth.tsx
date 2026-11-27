import { Keys } from "../ui/Keys";
import { prReviewKeys } from "../reviews/keys";
import { HoverCard, HoverCardHead, HoverCardLine, HoverCardStrong } from "../ui/HoverCard";
import { Icon, type IconName } from "../ui/Icon";
import { Text } from "../ui/Text";
import { type Tone, toneClass } from "../ui/tone";
import { cx } from "../ui/cx";
import css from "./ReviewHealth.module.css";

type ReviewHealthProps = {
  // Its checks as GitHub rolls them up: success, failure, pending, or ""
  // for none.
  checks?: string;
  // Whether it merges cleanly into its base: clean, conflicts, or ""
  // while GitHub hasn't said.
  merges?: string;
  // What it goes into, and who'd resolve a conflict: for the card's words.
  base: string;
  author?: string;
};

type Said = { icon: IconName; tone: Tone; title: string; why: React.ReactNode };

function checksSaid(checks = ""): Said {
  if (checks === "success") return { icon: "check-circle", tone: "done", title: "Checks pass", why: "Every check on the latest push passed." };
  if (checks === "failure")
    return {
      icon: "x-circle",
      tone: "danger",
      title: "Checks failing",
      why: (
        <>
          A check on the latest push failed. GitHub (<Keys k={prReviewKeys.github} />) says which, with its log.
        </>
      ),
    };
  if (checks === "pending") return { icon: "circle-dot", tone: "progress", title: "Checks running", why: "Some checks on the latest push haven't finished." };
  return { icon: "circle-dashed", tone: "quiet", title: "No checks", why: "Nothing's checked on its pushes." };
}

function mergesSaid(merges: string, base: string, author?: string): Said {
  const into = <Text mono>{base}</Text>;
  if (merges === "clean") return { icon: "merge", tone: "done", title: "No conflicts", why: <>It merges cleanly into {into}.</> };
  if (merges === "conflicts")
    return {
      icon: "merge",
      tone: "danger",
      title: `Conflicts with ${base}`,
      why: (
        <>
          It can't merge into {into} until {author ?? "its author"} resolves them.
        </>
      ),
    };
  return { icon: "merge", tone: "quiet", title: "Not known yet", why: <>GitHub hasn't said whether it merges cleanly into {into}: it works it out after a push, or when {base} moves.</> };
}

// ReviewHealth is whether a pull request can go in, in its bar: its checks
// and whether it conflicts with its base, an icon each in one pill, in
// the tone of how it stands. Pointed at, a card says each in words.
export function ReviewHealth({ checks, merges = "", base, author }: ReviewHealthProps) {
  const both = [checksSaid(checks), mergesSaid(merges, base, author)];
  return (
    <HoverCard
      className={css.health}
      trigger={
        <span className={css.pill} aria-label={both.map((s) => s.title).join("; ")}>
          <Icon name={both[0].icon} size={14} className={cx(css.mark, toneClass(both[0].tone))} />
          <span className={css.sep} />
          <Icon name={both[1].icon} size={14} className={cx(css.mark, toneClass(both[1].tone))} />
        </span>
      }
    >
      <span className={css.card}>
        {both.map((s) => (
          <span key={s.title} className={css.said}>
            <HoverCardHead>
              <Icon name={s.icon} size={14} className={cx(css.mark, toneClass(s.tone))} />
              <HoverCardStrong>{s.title}</HoverCardStrong>
            </HoverCardHead>
            <HoverCardLine dim wrap className={css.why}>
              {s.why}
            </HoverCardLine>
          </span>
        ))}
      </span>
    </HoverCard>
  );
}
