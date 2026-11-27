import { Button } from "../ui/Button";
import { Text } from "../ui/Text";
import css from "./RunsPageCache.module.css";

// RunsPageCache is what the cache made of a cached task's run, as facts
// in a RunsPageFacts: the pass it is, where it wasn't run (from, a run's
// number, and how long that took), with Run fresh beside it when it can be
// asked for; that it was kept, to be reused; why it couldn't be; and the
// files it changed that its outputs don't name.
export function RunsPageCache({
  reused,
  kept,
  keepFailed,
  undeclared,
  onFresh,
  busy,
}: {
  reused?: { from: string; took: string };
  kept?: boolean;
  keepFailed?: string;
  undeclared?: string[];
  onFresh?: () => void;
  busy?: boolean;
}) {
  return (
    <>
      {reused && (
        <>
          <dt>Reused</dt>
          <dd className={css.reused}>
            <span>
              from <Text mono>{reused.from}</Text>
              <Text dim>, which took {reused.took}: its inputs are the same</Text>
            </span>
            {onFresh && (
              <Button size="small" onPress={onFresh} disabled={busy} title="Run it, though a pass with the same inputs is kept">
                Run fresh
              </Button>
            )}
          </dd>
        </>
      )}
      {kept && (
        <>
          <dt>Cache</dt>
          <dd>
            Kept<Text dim>: a run with the same inputs reuses it</Text>
          </dd>
        </>
      )}
      {keepFailed && (
        <>
          <dt>Cache</dt>
          <dd>
            Not kept<Text dim>: {keepFailed}</Text>
          </dd>
        </>
      )}
      {(undeclared?.length ?? 0) > 0 && (
        <>
          <dt>Not in outputs</dt>
          <dd>
            <ul className={css.files}>
              {undeclared!.map((f) => (
                <Text as="li" mono key={f}>
                  {f}
                </Text>
              ))}
            </ul>
            <Text dim>It changed these as it ran. A reuse won't put them back: name them in outputs if it wrote them.</Text>
          </dd>
        </>
      )}
    </>
  );
}
