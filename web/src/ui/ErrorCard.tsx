import { Button } from "./Button";
import { Callout } from "./Callout";
import { humanError } from "./text";

// ErrorCard is something that didn't load, in words, with a way to try
// again where there is one: a Callout in danger, the error made a
// sentence.
export function ErrorCard({ error, retry, title = "Couldn't load this", className }: { error: string; retry?: () => void; title?: string; className?: string }) {
  return (
    <Callout tone="danger" title={title} className={className} actions={retry && <Button onClick={retry}>Try again</Button>}>
      {humanError(error)}
    </Callout>
  );
}
