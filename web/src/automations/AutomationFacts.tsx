import type { ReactNode } from "react";
import { Text } from "../ui/Text";
import { mayWords } from "./automationWords";
import css from "./AutomationFacts.module.css";

// AutomationFacts is when an automation runs (its when, as written) and
// what it may do, and any more rows after (children: dt/dd pairs).
export function AutomationFacts({ when, may, children }: { when?: string; may: string[]; children?: ReactNode }) {
  return (
    <dl className={css.facts}>
      <dt>When</dt>
      <dd>{when ? <Text mono>{when}</Text> : <Text dim>it can't be said</Text>}</dd>
      <dt>May</dt>
      <dd>{mayWords(may)}</dd>
      {children}
    </dl>
  );
}
