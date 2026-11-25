import type { ReactNode } from "react";
import { Button } from "../ui/Button";
import { cx } from "../ui/cx";
import { Input } from "../ui/Input";
import { Radio, RadioGroup } from "../ui/RadioGroup";
import css from "./SettingsAddRepoForm.module.css";

type SettingsAddRepoFormProps = {
  // Its name here, as typed; invalid where it can't be that, and what's
  // said under it either way.
  as: string;
  invalid?: boolean;
  hint: string;
  onAs: (text: string) => void;
  // Its group: a SettingsGroupPick.
  group: ReactNode;
  // Whose: the team's, or yours alone.
  team: boolean;
  onTeam: (team: boolean) => void;
  busy?: boolean;
  // What the button that adds it says.
  add: string;
  onCancel: () => void;
  onSubmit: () => void;
};

// SettingsAddRepoForm is the repo chosen, about to be added: its name
// here, its group, and whose it is.
export function SettingsAddRepoForm({ as, invalid, hint, onAs, group, team, onTeam, busy, add, onCancel, onSubmit }: SettingsAddRepoFormProps) {
  return (
    <form
      className={css.adding}
      onSubmit={(e) => {
        e.preventDefault();
        if (!invalid && !busy) onSubmit();
      }}
    >
      <div className={css.field}>
        <label htmlFor="add-repo-as">Its name here</label>
        <Input id="add-repo-as" className="mono" value={as} onChange={(e) => onAs(e.target.value)} aria-invalid={invalid} />
        <span className={cx("dim", css.hint)}>{hint}</span>
      </div>
      <div className={css.field}>
        <span className={css.label}>Its group</span>
        <span>{group}</span>
      </div>
      <fieldset className={css.field}>
        <legend>Whose</legend>
        <RadioGroup className={css.whose} value={team ? "team" : "me"} onChange={(v) => onTeam(v === "team")} aria-label="Whose">
          <Radio value="me">
            <span>
              Just me <span className="dim">— in ws.local.toml</span>
            </span>
          </Radio>
          <Radio value="team">
            <span>
              The team <span className="dim">— in ws.toml, committed and pushed</span>
            </span>
          </Radio>
        </RadioGroup>
      </fieldset>
      <div className={css.buttons}>
        <Button variant="ghost" type="button" onClick={onCancel} disabled={busy}>
          Cancel
        </Button>
        <Button variant="primary" type="submit" disabled={invalid || busy}>
          {add}
        </Button>
      </div>
    </form>
  );
}
