import { useNavigate } from "react-router";
import type { WhatsNewEntry as Entry, WhatsNewInfo } from "../../api";
import { shortVersion } from "../version";
import { ledeOf, releaseOf, releasesIn } from "./lede";
import { WhatsNewDialog } from "./WhatsNewDialog";
import { WhatsNewEntry } from "./WhatsNewEntry";
import { WhatsNewGrid } from "./WhatsNewGrid";

// News is what's new, as the dialog draws it: the first entry large and
// the rest as cards, and a way to each where it has one, which closes the
// dialog on the way.
export function News({ title, entries, info, onClose }: { title: string; entries: Entry[]; info: WhatsNewInfo; onClose: () => void }) {
  const navigate = useNavigate();
  const several = releasesIn(entries) > 1;
  const [lead, ...rest] = entries;
  const show = (link?: string) =>
    link
      ? () => {
          onClose();
          void navigate(link);
        }
      : undefined;
  const entry = (e: Entry, isLead?: boolean) => (
    <WhatsNewEntry key={e.slug} lead={isLead} picture={e.picture} title={e.title} body={e.body} release={several ? releaseOf(e) : undefined} onShow={show(e.link)} />
  );
  return (
    <WhatsNewDialog title={title} version={shortVersion(info.version)} lede={ledeOf(entries, info)} releases={info.missing ? info.releases : undefined} onClose={onClose}>
      {!lead ? (
        <p className="dim">Nothing's said to be new in this ws.</p>
      ) : (
        <>
          {entry(lead, true)}
          {rest.length > 0 && <WhatsNewGrid>{rest.map((e) => entry(e))}</WhatsNewGrid>}
        </>
      )}
    </WhatsNewDialog>
  );
}
