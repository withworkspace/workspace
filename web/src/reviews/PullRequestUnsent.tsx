import type { Unsent } from "../api";
import { Banner } from "../ui/Banner";
import { Button } from "../ui/Button";
import { capital, clip, humanError } from "../ui/strings";
import css from "./PullRequestUnsent.module.css";

type Props = {
  unsent: Unsent[];
  // One's being sent again, or let go of: its buttons wait.
  busy?: number;
  onResend: (u: Unsent) => void;
  onDiscard: (u: Unsent) => void;
};

// PullRequestUnsent is what was said from a pull request's page that
// GitHub didn't take (a reply, a comment, resolving a thread, your review
// of it): each with why, and to send again or let go of. A review isn't
// let go of: it's made your draft again. What's said here is shown at once
// and sent behind, so a refusal comes after the page has moved on: this
// is where it's said, until something's done about it.
export function PullRequestUnsent({ unsent, busy, onResend, onDiscard }: Props) {
  if (unsent.length === 0) return null;
  return (
    <div className={css.unsent}>
      {unsent.map((u) => (
        <Banner
          key={u.id}
          icon="alert"
          actions={
            <>
              <Button size="small" disabled={busy === u.id} onClick={() => onResend(u)} title="Send to GitHub again">
                Try again
              </Button>
              <Button
                variant="ghost"
                size="small"
                disabled={busy === u.id}
                onClick={() => onDiscard(u)}
                title={u.draft ? "Restore as draft, with comments" : "Discard without sending"}
              >
                {u.draft ? "Restore draft" : "Discard"}
              </Button>
            </>
          }
        >
          {capital(u.doing)} did not reach GitHub: {humanError(u.error)}
          {u.body && <span className={css.said}> “{clip(u.body, 140)}”</span>}
        </Banner>
      ))}
    </div>
  );
}
