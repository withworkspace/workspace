import { useEffect, useId, useMemo, useRef, useState } from "react";
import type { ModelInfo } from "../api";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Input } from "../ui/Input";
import { Spinner } from "../ui/Spinner";
import { choiceOf, find, type ModelChoice, pick, rowOf, treeOf } from "./modelTree";
import { ModelChoices } from "./ModelChoices";
import css from "./ModelPanel.module.css";
import { ModelRow } from "./ModelRow";

const nothing: ModelChoice = { effort: "", thinking: false, fast: false };

// ModelPanel is a model picker's inside (ModelPicker, ModelSwitch): a text box over a
// harness's models, a row for each model (its names together: each
// effort, thinking or not, fast or not), under its maker; and under the
// list, what's chosen of the model the cursor's on, and Use.
//
// Typing finds models by every word ("opus 5.5"); a word that's a choice
// ("high", "fast", "thinking") chooses it instead. ↓ and ↑ (Ctrl+N, P)
// move the cursor, a press puts it on a row, and ⏎, Use or a double
// press uses what it comes to. The focus stays in the text box, which
// says which row the cursor's on, as a combobox does. The cursor starts
// on what's set (value), at what it chooses.
//
// It's driven by hand, as Finder is, for the same reasons: react-aria's
// autocomplete shows no cursor till a key's pressed.
export function ModelPanel({
  models,
  value,
  busy,
  onUse,
  onAnother,
}: {
  models: ModelInfo[];
  value: string;
  busy?: boolean;
  onUse: (id: string) => void;
  onAnother?: () => void;
}) {
  const id = useId();
  const list = useRef<HTMLUListElement>(null);
  const tree = useMemo(() => treeOf(models), [models]);
  const set = models.find((m) => m.id === value);
  const [query, setQuery] = useState("");
  const [chosen, setChosen] = useState<ModelChoice>(set ? choiceOf(set) : nothing);
  // The choices said, by a word typed or a control, the last first.
  const [said, setSaid] = useState<(keyof ModelChoice)[]>([]);
  const found = useMemo(() => find(tree, query, chosen).tree, [tree, query, chosen]);
  const rows = found.flatMap((s) => s.rows);
  const [cursor, setCursor] = useState(() => Math.max(0, rows.findIndex((r) => r === rowOf(tree, value))));
  const at = Math.min(cursor, Math.max(0, rows.length - 1));
  const row = rows[at];
  const model = row && pick(row, chosen, said);
  useEffect(() => {
    list.current?.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: "nearest" });
  }, [at, query]);
  const use = () => model && onUse(model.id);
  const move = (d: number) => setCursor(Math.max(0, Math.min(at + d, rows.length - 1)));
  let n = 0;
  return (
    <div className={css.panel}>
      <div className={css.search}>
        <Icon name="search" size={15} />
        <Input
          autoFocus
          value={query}
          placeholder="Find a model, or type high, fast…"
          aria-label="Find a model"
          role="combobox"
          aria-expanded="true"
          aria-controls={`${id}-list`}
          aria-activedescendant={row ? `${id}-${at}` : undefined}
          spellCheck={false}
          autoComplete="off"
          className={css.input}
          onChange={(e) => {
            const q = e.target.value;
            setQuery(q);
            setCursor(0);
            const words = find(tree, q, chosen);
            if (words.said.length) {
              setChosen(words.choice);
              setSaid([...words.said, ...said.filter((x) => !words.said.includes(x))]);
            }
          }}
          onKeyDown={(e) => {
            if (e.key === "ArrowDown" || (e.ctrlKey && e.key === "n")) {
              e.preventDefault();
              move(1);
            } else if (e.key === "ArrowUp" || (e.ctrlKey && e.key === "p")) {
              e.preventDefault();
              move(-1);
            } else if (e.key === "Enter") {
              e.preventDefault();
              use();
            }
          }}
        />
        {busy && <Spinner size={14} />}
      </div>
      <ul className={css.list} id={`${id}-list`} ref={list} role="listbox" aria-label="Models">
        {found.map((s) => (
          <li key={s.vendor} role="presentation" className={css.section}>
            {s.label && (
              <div className={css.head} id={`${id}-${s.vendor}`} aria-hidden>
                {s.label}
              </div>
            )}
            <ul role="group" aria-labelledby={s.label ? `${id}-${s.vendor}` : undefined} aria-label={s.label ? undefined : "Others"} className={css.group}>
              {s.rows.map((r) => {
                const i = n++;
                return (
                  <ModelRow
                    key={r.key}
                    id={`${id}-${i}`}
                    row={r}
                    on={i === at}
                    chosen={r.models.some((m) => m.id === value)}
                    onPress={() => setCursor(i)}
                    onUse={() => onUse(pick(r, chosen, said).id)}
                  />
                );
              })}
            </ul>
          </li>
        ))}
        {!rows.length && (
          <li role="presentation" className={css.none}>
            {busy ? "Loading models…" : "No matching models."}
          </li>
        )}
      </ul>
      {row && model && (
        <ModelChoices
          row={row}
          model={model}
          onUse={use}
          onChoose={(on, v) => {
            setChosen({ ...chosen, [on]: v });
            setSaid([on, ...said.filter((x) => x !== on)]);
          }}
        />
      )}
      {onAnother && (
        <div className={css.foot}>
          <Button variant="link" size="small" onClick={onAnother}>
            Another, by name…
          </Button>
        </div>
      )}
    </div>
  );
}
