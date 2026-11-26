import type { ReactNode } from "react";
import { PageHead } from "../ui/PageHead";
import { Segment, Segmented } from "../ui/Segmented";
import { Option, Select } from "../ui/Select";
import { ANYWHERE, BY_HAND, type Shown, shownLabels, type Source } from "./shown";

type ArtifactsHeadProps = {
  // filters is whether there's anything to filter: a list with none has
  // its title alone.
  filters: boolean;
  // Which are shown, by where they stand, and how many are discarded.
  shown: Shown;
  onShown: (shown: Shown) => void;
  discarded: number;
  // Where they came from: the places there are, and the one picked.
  froms: Source[];
  from: string;
  onFrom: (from: string) => void;
  // What's pressed after the filters: Clear expired.
  children?: ReactNode;
};

// ArtifactsHead is the head of the Artifacts page: its title, and the
// list's filters, by where an artifact stands and where it came from.
export function ArtifactsHead({ filters, shown, onShown, discarded, froms, from, onFrom, children }: ArtifactsHeadProps) {
  return (
    <PageHead
      title="Artifacts"
      actions={
        filters && (
          <>
            <Segmented value={shown} onChange={onShown} aria-label="Status">
              {(Object.keys(shownLabels) as Shown[]).map((s) => (
                <Segment key={s} value={s}>
                  {shownLabels[s]}
                  {s === "discarded" && discarded > 0 ? ` ${discarded}` : ""}
                </Segment>
              ))}
            </Segmented>
            {froms.length > 0 && (
              <Select value={from} onChange={onFrom} aria-label="Origin">
                <Option value={ANYWHERE}>From anywhere</Option>
                <Option value={BY_HAND}>Added by hand</Option>
                {froms.map((f) => (
                  <Option key={f.from} value={f.from} title={f.title}>
                    {f.title ? `${f.from}: ${f.title}` : f.from}
                  </Option>
                ))}
              </Select>
            )}
            {children}
          </>
        )
      }
    />
  );
}
