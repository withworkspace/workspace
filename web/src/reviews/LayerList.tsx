import type { PRDetail } from "../api";
import { PullRequestLayers } from "./PullRequestLayers";
import { prHref, prStatus } from "./Reviews";

// LayerList is a stack's layers down the side of a pull request's review,
// as PullRequestLayers. going is the layer (or the whole stack) asked for
// and being read, to show it's coming; search is the address's, kept
// going from one to another.
export function LayerList({ d, whole, going, search, onWhole }: { d: PRDetail; whole: boolean; going?: { number: number; whole: boolean }; search: string; onWhole: () => void }) {
  const st = d.stack!;
  return (
    <PullRequestLayers
      layers={st.layers.map((l) => ({
        number: l.number,
        title: l.title,
        words: l.words,
        status: prStatus(l),
        to: prHref(l.repo, l.number) + search,
        here: !whole && l.number === d.pr.number,
        going: !!going && !going.whole && going.number === l.number,
      }))}
      merged={st.merged}
      title={st.workedOut ? "Detected stack: each pull request builds on the one before" : `Stack ${st.number} on GitHub`}
      about={`${st.layers.length} ${st.layers.length === 1 ? "layer" : "layers"}${st.workedOut ? " · detected" : ""}`}
      whole={whole}
      goingWhole={going?.whole}
      onWhole={onWhole}
    />
  );
}
