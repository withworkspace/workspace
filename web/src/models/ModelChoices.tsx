import type { ModelInfo } from "../api";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Segment, Segmented } from "../ui/Segmented";
import { Switch } from "../ui/Switch";
import { choiceOf, effortWords, type ModelChoice, type ModelRow } from "./modelTree";
import css from "./ModelChoices.module.css";

// ModelChoices is what a model picker chooses of the model the
// cursor's on: its effort, thinking and fast, each where the model has
// more than one; then the name that comes to (model), and Use, which sets
// it. A choice the model has only with others changes those too (the
// picker's to work out, onChoose).
export function ModelChoices({
  row,
  model,
  onChoose,
  onUse,
}: {
  row: ModelRow;
  model: ModelInfo;
  onChoose: <K extends keyof ModelChoice>(on: K, value: ModelChoice[K]) => void;
  onUse: () => void;
}) {
  const c = choiceOf(model);
  return (
    <div className={css.choices}>
      {(row.efforts.length > 1 || row.thinking.length > 1 || row.fast.length > 1) && (
        <div className={css.controls}>
          {row.efforts.length > 1 && (
            <Segmented size="small" value={c.effort} onChange={(e) => onChoose("effort", e)} aria-label="Effort" className={css.effort}>
              {row.efforts.map((e) => (
                <Segment key={e} value={e} title={e ? undefined : "The model's own effort"}>
                  {effortWords[e]}
                </Segment>
              ))}
            </Segmented>
          )}
          <span className={css.toggles}>
            {row.thinking.length > 1 && (
              <span className={css.toggle}>
                <Icon name="sparkle" size={13} className={css.icon} />
                <span aria-hidden>Thinking</span>
                <Switch on={c.thinking} onChange={(on) => onChoose("thinking", on)} aria-label="Thinking" />
              </span>
            )}
            {row.fast.length > 1 && (
              <span className={css.toggle}>
                <Icon name="bolt" size={13} className={css.icon} />
                <span aria-hidden>Fast</span>
                <Switch on={c.fast} onChange={(on) => onChoose("fast", on)} aria-label="Fast" />
              </span>
            )}
          </span>
        </div>
      )}
      <div className={css.use}>
        <code className={css.id} title={model.name}>
          {model.id}
        </code>
        <Button size="small" variant="primary" onClick={onUse} keys="⏎">
          Use
        </Button>
      </div>
    </div>
  );
}
