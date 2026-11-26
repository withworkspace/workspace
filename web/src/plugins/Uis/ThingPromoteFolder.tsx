import { chipClass } from "../../ui/Chip";
import { Input } from "../../ui/Input";
import css from "./ThingPromoteFolder.module.css";
import { Text, textClass } from "../../ui/Text";

type ThingPromoteFolderProps = {
  // The place's name, as the field says it.
  label: string;
  // What's typed, and the folder it comes to.
  under: string;
  group: string;
  onUnder: (under: string) => void;
  // The folders its kind's things are grouped in there now.
  folders: string[];
  busy: boolean;
};

// ThingPromoteFolder is a folder of the place picked, to group the thing
// in: a field, and the folders in use there to pick from.
export function ThingPromoteFolder({ label, under, group, onUnder, folders, busy }: ThingPromoteFolderProps) {
  return (
    <div className={css.field}>
      <label>
        <Text dim>Folder (optional)</Text>
        <Input type="text" className={textClass("mono")} value={under} placeholder="cross-stack" aria-label={`Folder in ${label}`} list="thing-ui-groups" disabled={busy} onChange={(e) => onUnder(e.currentTarget.value)} />
      </label>
      <datalist id="thing-ui-groups">
        {folders.map((g) => (
          <option key={g} value={g} />
        ))}
      </datalist>
      {folders.length > 0 && (
        <div className={css.folders} role="group" aria-label="Existing folders">
          {folders.slice(0, 8).map((g) => (
            <button key={g} type="button" className={chipClass(undefined, css.folder)} aria-pressed={group === g} disabled={busy} onClick={() => onUnder(group === g ? "" : g)}>
              {g}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
