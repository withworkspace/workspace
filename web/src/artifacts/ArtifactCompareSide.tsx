import type { ReactNode } from "react";
import { buttonClass } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Option, Select } from "../ui/Select";
import css from "./ArtifactCompareSide.module.css";

type ArtifactCompareSideProps = {
  // Which artifact it is, of those it can be, and what picking another
  // does. label names the dropdown: "The left one's artifact".
  label: string;
  picked: string;
  choices: { id: string; title: string }[];
  onPick: (id: string) => void;
  // Its versions' strip.
  versions: ReactNode;
  // The button that opens its tweaks, where its frame's page has some.
  tweaks?: ReactNode;
  // Where its version runs alone, in a tab of its own.
  alone?: string;
  // The version's line, and its note, under the tools.
  children?: ReactNode;
};

// ArtifactCompareSide is one side's head: which artifact (any of the
// list), its versions, its tweaks and Open alone, then the version's line
// and note.
export function ArtifactCompareSide({ label, picked, choices, onPick, versions, tweaks, alone, children }: ArtifactCompareSideProps) {
  return (
    <div className={css.side}>
      <div className={css.tools}>
        <Select className={css.pick} value={picked} onChange={onPick} aria-label={label}>
          {choices.map((a) => (
            <Option key={a.id} value={a.id} title={a.title}>
              {`${a.id} · ${a.title}`}
            </Option>
          ))}
        </Select>
        {versions}
        <span className={css.end}>
          {tweaks}
          {alone && (
            <a className={buttonClass({ variant: "ghost" }, css.alone)} href={alone} target="_blank" rel="noopener noreferrer" title="In new tab, still sandboxed">
              <Icon name="external" size={13} /> Open in new tab
            </a>
          )}
        </span>
      </div>
      {children}
    </div>
  );
}
