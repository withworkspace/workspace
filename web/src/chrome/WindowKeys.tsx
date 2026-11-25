import css from "./WindowKeys.module.css";
import { WindowKeyLabel } from "./WindowKeyLabel";

// A group of keys: its heading, and each key with what it does.
export interface WindowKeysGroup {
  title: string;
  keys: [string, string][];
}

// WindowKeys is the keys, in the Keys dialog: a group each (those of the
// page, those everywhere), each key's label beside what it does.
export function WindowKeys({ groups }: { groups: WindowKeysGroup[] }) {
  return (
    <div className={css.keys}>
      {groups.map((g) => (
        <div key={g.title}>
          <h3>{g.title}</h3>
          <dl>
            {g.keys.map(([k, what]) => (
              <div key={k}>
                <dt>
                  <WindowKeyLabel k={k} />
                </dt>
                <dd>{what}</dd>
              </div>
            ))}
          </dl>
        </div>
      ))}
    </div>
  );
}
