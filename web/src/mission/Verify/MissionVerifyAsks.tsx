import { Button } from "../../ui/Button";
import { Inline } from "../../ui/Inline";
import css from "./MissionVerifyAsks.module.css";

// MissionVerifyAsks is what a bundle's crew suggest is done about a
// check. While the bundle's yours to settle each is a button (onAsk),
// which takes it as what you say isn't right; after, they're a list.
export function MissionVerifyAsks({ crew, asks, onAsk }: { crew: string; asks: string[]; onAsk?: (ask: string) => void }) {
  if (!onAsk) {
    return (
      <div>
        <h4 className={css.head}>{crew} suggested</h4>
        <ul className={css.list}>
          {asks.map((a, i) => (
            <li key={i}>
              <Inline text={a} />
            </li>
          ))}
        </ul>
      </div>
    );
  }
  return (
    <div>
      <h4 className={css.head}>{crew} suggests, to send back</h4>
      <div className={css.chips}>
        {asks.map((a, i) => (
          <Button key={i} type="button" className={css.ask} onClick={() => onAsk(a)} title="Add as not right">
            {a}
          </Button>
        ))}
      </div>
    </div>
  );
}
