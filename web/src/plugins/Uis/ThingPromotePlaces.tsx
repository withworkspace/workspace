import { Radio, RadioGroup } from "../../ui/RadioGroup";
import css from "./ThingPromotePlaces.module.css";
import type { ThingPromotePlace } from "./ThingPromote";

type ThingPromotePlacesProps = {
  places: ThingPromotePlace[];
  // Which of them, or somewhere else, said in words.
  pick: number | "else";
  onPick: (pick: number | "else") => void;
  busy: boolean;
};

// ThingPromotePlaces is where a thing could live, where its kind's things
// go in more places than one: each by its repo, with where in it, to pick
// one of, or somewhere else.
export function ThingPromotePlaces({ places, pick, onPick, busy }: ThingPromotePlacesProps) {
  return (
    <fieldset className={css.places} disabled={busy}>
      <legend>Location</legend>
      <RadioGroup className={css.list} value={String(pick)} onChange={(v) => onPick(v === "else" ? "else" : Number(v))} disabled={busy} aria-label="Location">
        {places.map((p, i) => (
          <Radio key={p.key} value={String(i)}>
            <span className={css.place}>
              <span className={css.repo}>{p.label}</span>
              <code>{p.where}</code>
            </span>
          </Radio>
        ))}
        <Radio value="else">Somewhere else…</Radio>
      </RadioGroup>
    </fieldset>
  );
}
