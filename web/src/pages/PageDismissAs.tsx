import { useState } from "react";
import type { DismissWhy } from "../api";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import css from "./PageDismissAs.module.css";
import { Text } from "../ui/Text";

// PageDismissAs dismisses a finding, saying why (whys: each reason, its
// words and what it means): it's wrong, the page already says it, or it's
// so but not worth acting on. How often each is how well a pass does
// (ws show passes).
export function PageDismissAs({ whys, onDismiss }: { whys: [DismissWhy, string, string][]; onDismiss: (why: DismissWhy) => void }) {
  const [asking, setAsking] = useState(false);
  if (!asking)
    return (
      <Button variant="ghost" size="small" onClick={() => setAsking(true)} title="Not an issue: don't raise again">
        Dismiss
      </Button>
    );
  return (
    <span className={css.as} role="group" aria-label="Dismiss as">
      <Text dim>Dismiss as</Text>
      {whys.map(([why, label, title]) => (
        <Button variant="ghost" size="small" key={why} title={title} onClick={() => onDismiss(why)}>
          {label}
        </Button>
      ))}
      <Button variant="ghost" size="small" square onClick={() => setAsking(false)} aria-label="Cancel" title="Cancel">
        <Icon name="x" size={11} />
      </Button>
    </span>
  );
}
