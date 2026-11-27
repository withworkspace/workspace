import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import { cx } from "../ui/cx";
import css from "./PullRequestMerge.module.css";

type Props = {
  // What's offered: merging it now; leaving it to merge by itself once it
  // can; or, set to, stopping that.
  offer: "now" | "ready" | "on";
  // A stack, which merges whole: said on the button.
  stack?: boolean;
  // It's being asked: the button waits.
  busy?: boolean;
  // Accented: the one thing the place it's in asks of you.
  primary?: boolean;
  className?: string;
  onMerge: () => void;
  onStop?: () => void;
};

// PullRequestMerge is merging a pull request of yours from ws: a stack
// whole, never in part. Ready, it merges; held only by what's to be waited
// for (its checks, a review, its base's rules), it can be left to merge by
// itself once it can; and left to, it says so, with the way to stop it.
export function PullRequestMerge({ offer, stack, busy, primary, className, onMerge, onStop }: Props) {
  const what = stack ? "stack" : "";
  if (offer === "on") {
    return (
      <span className={cx(css.merge, className)}>
        <span className={css.set} title="Auto-merge is on: merges once checks and reviews pass">
          <Icon name="merge" size={13} /> Merges when ready
        </span>
        {onStop && (
          <Button variant="ghost" size="small" disabled={busy} onClick={onStop} title="Cancel auto-merge">
            Cancel
          </Button>
        )}
      </span>
    );
  }
  const ready = offer === "ready";
  return (
    <span className={cx(css.merge, className)}>
      <Button
        variant={primary && !ready ? "primary" : undefined}
        size="small"
        disabled={busy}
        onClick={onMerge}
        title={ready ? "Merge once checks and reviews pass" : stack ? "Merge whole stack on GitHub" : "Merge on GitHub"}
      >
        {busy ? <Spinner size={12} /> : <Icon name="merge" size={13} />} {ready ? "Merge when ready" : what ? `Merge ${what}` : "Merge"}
      </Button>
    </span>
  );
}
