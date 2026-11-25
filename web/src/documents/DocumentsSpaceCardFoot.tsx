import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import { Time } from "../ui/Time";
import css from "./DocumentsSpaceCardFoot.module.css";

// How a space stands with its remote, as its card says it: not cloned
// here, behind, up to date (fetched when, if ever), or not read yet.
export type SpaceStands = { cloned: false; repo: string } | { behind: number } | { fetched?: string } | undefined;

// DocumentsSpaceCardFoot is how a space stands, at its card's foot, and
// how many of its documents are in flight, and of yours are drafts in it.
export function DocumentsSpaceCardFoot({ stands, inFlight, drafts }: { stands: SpaceStands; inFlight: number; drafts: number }) {
  return (
    <footer className={css.foot}>
      <span>
        {stands && "cloned" in stands ? (
          <>
            <Icon name="box" size={12} /> <span className="mono">ws setup {stands.repo}</span> clones it
          </>
        ) : stands && "behind" in stands ? (
          <span className={css.behind}>
            <Icon name="alert" size={12} /> {stands.behind} {stands.behind === 1 ? "commit" : "commits"} behind
          </span>
        ) : stands ? (
          <>
            <Icon name="check" size={12} /> Up to date
            {stands.fetched && (
              <>
                {" "}
                · fetched <Time iso={stands.fetched} as="age" /> ago
              </>
            )}
          </>
        ) : null}
      </span>
      {inFlight > 0 && <Chip tone="progress">{inFlight} in flight</Chip>}
      {drafts > 0 && <Chip tone="attention">{drafts === 1 ? "1 draft of yours" : `${drafts} drafts of yours`}</Chip>}
    </footer>
  );
}
