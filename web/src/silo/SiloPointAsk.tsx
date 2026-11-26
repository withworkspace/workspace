import { Button } from "../ui/Button";
import { Empty } from "../ui/Empty";
import { Icon } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import css from "./SiloPointAsk.module.css";

type SiloPointAskProps = {
  repo: string;
  // The capsule to point it at; with none, the page says what it's for.
  to: string;
  // It goes on somewhere once it's pointed.
  then?: boolean;
  // It's being pointed.
  busy?: boolean;
  // Why it couldn't be.
  error?: string;
  onPoint: () => void;
  onNotNow: () => void;
};

// SiloPointAsk asks whether to point a repo's silo at a capsule: what
// that does, and the two ways on, in the middle of the page.
export function SiloPointAsk({ repo, to, then, busy, error, onPoint, onNotNow }: SiloPointAskProps) {
  return (
    <div className={css.point}>
      <Empty
        icon="layers"
        title={to ? `Point ${repo}'s silo at ${to}?` : "Which capsule?"}
        action={
          to && (
            <div className={css.acts}>
              <Button variant="primary" disabled={busy} onClick={onPoint} autoFocus>
                {busy ? <Spinner size={12} /> : <Icon name="arrow-right" size={12} />} {busy ? "Pointing…" : `Point at ${to}`}
              </Button>
              <Button variant="ghost" disabled={busy} onClick={onNotNow}>
                Not now
              </Button>
            </div>
          )
        }
      >
        {to ? (
          <>
            Silo's files become {to}'s, and its after_switch hook restarts what's running.
            {then ? " Then you go there." : ""}{error && <p className={css.error}>{error}</p>}
          </>
        ) : (
          "Points a silo at a capsule. Open this page from the router, with a capsule named."
        )}
      </Empty>
    </div>
  );
}
