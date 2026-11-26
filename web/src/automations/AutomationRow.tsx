import { Link } from "react-router";
import type { AutomationItem } from "../api";
import { Row } from "../ui/Rows";
import { StatusIcon } from "../ui/StatusIcon";
import { Switch } from "../ui/Switch";
import { Text } from "../ui/Text";
import { Time } from "../ui/Time";
import { runLook, standing, tallyWords } from "./automationWords";
import css from "./AutomationRow.module.css";

// AutomationRow is one automation on the Automations page: its name (to
// its page) and your words for it, when it runs, how its last run went
// and how its runs stand, or what's wrong with it; and its switch, on or
// off. One that's off is quieter.
export function AutomationRow({ a, onOn, busy }: { a: AutomationItem; onOn: (on: boolean) => void; busy?: boolean }) {
  const wrong = standing(a);
  const last = a.last;
  const look = last && runLook(last.state);
  const tally = tallyWords(a.tally);
  return (
    <Row className={a.on ? css.row : `${css.row} ${css.off}`}>
      <div className={css.main}>
        <div className={css.top}>
          <Link to={`/automations/${encodeURIComponent(a.name)}`} className={css.name}>
            {a.name}
          </Link>
          {!a.on && <Text dim>off</Text>}
        </div>
        {a.about && <div className={css.about}>{a.about}</div>}
        {a.when && (
          <Text as="div" mono dim className={css.when}>
            {a.when}
          </Text>
        )}
        {wrong ? (
          <div className={css.wrong}>{wrong.words}</div>
        ) : (
          <div className={css.last}>
            {last && look ? (
              <>
                <StatusIcon icon={look.icon} tone={look.tone} size={12} title={look.words} />
                <span>
                  {look.words} <Time iso={last.at} as="relative" />
                  {last.says && <Text dim> · {last.says}</Text>}
                </span>
              </>
            ) : (
              <Text dim>Hasn't run yet</Text>
            )}
            {tally.length > 0 && <Text dim className={css.tally}>{tally.join(" · ")}</Text>}
          </div>
        )}
      </div>
      <Switch on={a.on} onChange={onOn} disabled={busy || !!a.broken} aria-label={`${a.name} on`} className={css.switch} />
    </Row>
  );
}
