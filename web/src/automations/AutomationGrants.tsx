import type { AutomationKind } from "../api";
import { Switch } from "../ui/Switch";
import { Text } from "../ui/Text";
import css from "./AutomationGrants.module.css";

// AutomationGrants is what an automation may do through plugins (act),
// and whether it does it without asking: a switch each. Off, what it does
// is proposed to you first. A kind its plugin doesn't say is safe done
// with nobody looking can't be switched on: it's always proposed. Runs you
// start yourself put it in front of you to confirm either way.
export function AutomationGrants({ kinds, busy, onGrant }: { kinds: AutomationKind[]; busy?: boolean; onGrant: (kind: string, on: boolean) => void }) {
  return (
    <ul className={css.grants}>
      {kinds.map((k) => (
        <li key={k.kind} className={css.grant}>
          <Switch on={!!k.granted} onChange={(on) => onGrant(k.kind, on)} disabled={busy || (!k.unattended && !k.granted)} aria-label={`${k.kind} without asking`} />
          <div className={css.words}>
            <span>
              {k.title || k.kind} <Text mono dim>{k.kind}</Text>
            </span>
            <Text dim className={css.note}>
              {k.missing
                ? "No plugin here proposes it: it can't be done."
                : !k.unattended
                  ? "Always proposed to you: its plugin doesn't say it's safe done with nobody looking."
                  : k.granted
                    ? "Done without asking, as it runs on what happens."
                    : "Proposed to you, to accept, as it runs on what happens."}
            </Text>
          </div>
        </li>
      ))}
    </ul>
  );
}
