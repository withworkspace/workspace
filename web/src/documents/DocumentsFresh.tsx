import { Button } from "../ui/Button";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import { Time } from "../ui/Time";
import css from "./DocumentsFresh.module.css";

type DocumentsFreshProps = {
  fetching?: boolean; // it's being fetched
  behind: number; // commits behind its remote
  remote: string; // origin/<branch>
  dirty?: boolean; // its ground has changes of its own, so it isn't pulled
  pulling?: boolean;
  onPull: () => void;
  fetched?: string; // when it was last fetched
};

// DocumentsFresh is how a space's ground stands with its remote: being
// fetched, behind (and Pull, if it can be), or up to date; and when it
// was fetched.
export function DocumentsFresh({ fetching, behind, remote, dirty, pulling, onPull, fetched }: DocumentsFreshProps) {
  return (
    <p className={cx(css.fresh, "dim", !!behind && css.behind)}>
      {fetching ? (
        <>
          <Spinner size={12} /> Fetching…
        </>
      ) : behind ? (
        <>
          <Icon name="alert" size={12} /> {behind} {behind === 1 ? "commit" : "commits"} behind {remote}
          {dirty ? (
            <span> · its ground has changes of its own, so it isn't moved</span>
          ) : (
            <Button size="small" onClick={onPull} disabled={pulling}>
              {pulling ? "Pulling…" : "Pull"}
            </Button>
          )}
        </>
      ) : (
        <>
          <Icon name="check" size={12} /> Up to date with {remote}
        </>
      )}
      {fetched && !fetching && (
        <span>
          {" "}
          · fetched <Time iso={fetched} as="age" /> ago
        </span>
      )}
    </p>
  );
}
