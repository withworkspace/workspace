import type { Ref } from "react";
import { Button } from "../../ui/Button";
import { Input } from "../../ui/Input";
import css from "./ThingPromote.module.css";
import { ThingPromoteFolder } from "./ThingPromoteFolder";
import { ThingPromotePlaces } from "./ThingPromotePlaces";
import { ThingUi, ThingUiButtons, ThingUiFailed, ThingUiTitle } from "./ThingUi";

// ThingPromotePlace is a place a thing could live.
export type ThingPromotePlace = {
  key: string;
  // Its repo, by name.
  label: string;
  // Where in it, as it would be: in the folder said, for the one picked.
  where: string;
  // It takes no pull requests: the thing's put on its default branch.
  direct: boolean;
  // Where its things may be grouped in a folder of it: those in use.
  folders?: string[];
};

type ThingPromoteProps = {
  // The thing, by its title or its ref.
  what: string;
  // Where it could live: none while that's worked out.
  places?: ThingPromotePlace[];
  // Which of them, or somewhere else, said in words.
  pick: number | "else";
  onPick: (pick: number | "else") => void;
  // A folder of the place picked, as typed and as it comes to.
  under: string;
  group: string;
  onUnder: (under: string) => void;
  // Somewhere else, as typed.
  elsewhere: string;
  onElsewhere: (to: string) => void;
  // It's being put there.
  busy: boolean;
  // Why it couldn't be.
  failed?: string;
  // There's somewhere to put it.
  ready: boolean;
  go?: Ref<HTMLButtonElement>;
  onPromote: () => void;
  onCancel: () => void;
};

// ThingPromote is what puts one of a plugin's own things in a repo: where
// it would live, or the places to pick from; a folder there; somewhere
// else in words; and why it couldn't be.
export function ThingPromote({ what, places, pick, onPick, under, group, onUnder, elsewhere, onElsewhere, busy, failed, ready, go, onPromote, onCancel }: ThingPromoteProps) {
  const several = (places?.length ?? 0) > 1;
  const place = pick === "else" ? undefined : places?.[pick];
  return (
    <ThingUi onSubmit={() => !busy && onPromote()}>
      <ThingUiTitle>Put {what} in a repo</ThingUiTitle>
      <p>Everyone with the repo has it then. What you change after is on this machine until you save it there.</p>
      {!places && <p className={css.where}>Working out where it would live…</p>}
      {places && !several && pick !== "else" && place && (
        <p className={css.where}>
          It'll live at <code>{place.where}</code>
          <Button variant="link" type="button" className={css.else} disabled={busy} onClick={() => onPick("else")}>
            Somewhere else…
          </Button>
        </p>
      )}
      {places && several && <ThingPromotePlaces places={places} pick={pick} onPick={onPick} busy={busy} />}
      {place?.folders && <ThingPromoteFolder label={place.label} under={under} group={group} onUnder={onUnder} folders={place.folders} busy={busy} />}
      {places && pick === "else" && (
        <label className={css.where}>
          {!several && <span className="dim">It'll live at</span>}
          {/* biome-ignore lint/a11y/noAutofocus: the field is what was asked for */}
          <Input type="text" className="mono" value={elsewhere} placeholder="repo:folder/" aria-label="Where it'll live, as repo:folder/" autoFocus disabled={busy} onChange={(e) => onElsewhere(e.currentTarget.value)} />
        </label>
      )}
      {place && !place.direct && <p className="dim">{place.label} takes pull requests: it'll be left in a capsule, for you to open one from.</p>}
      {failed && <ThingUiFailed>{failed}</ThingUiFailed>}
      <ThingUiButtons>
        <Button variant="primary" size="small" type="submit" ref={go} disabled={busy || !ready}>
          {busy ? "Promoting…" : "Promote"}
        </Button>
        <Button variant="ghost" size="small" type="button" disabled={busy} onClick={onCancel} keys="esc">
          Cancel
        </Button>
      </ThingUiButtons>
    </ThingUi>
  );
}
