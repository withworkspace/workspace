import { Button } from "./Button";
import { Callout } from "./Callout";
import { humanError } from "./strings";

// ErrorCard is something that didn't load, in words, with a way to try
// again where there is one: a Callout in danger, the error made a
// sentence.
//
// keys is the key that tries again too, where the page has one (r, its
// reload): the button says it, and hints at it (ui/keyHint.ts).
export function ErrorCard({ error, retry, keys, title = "Could not load", className }: { error: string; retry?: () => void; keys?: string; title?: string; className?: string }) {
  return (
    <Callout tone="danger" title={title} className={className} actions={
        retry && (
          <Button onClick={retry} keys={keys}>
            Try again
          </Button>
        )
      }>
      {humanError(error)}
    </Callout>
  );
}
