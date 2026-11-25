import { useEffect, useRef } from "react";
import { getMission, type MissionDetail } from "../api";
import { DockBody } from "../chrome/DockBody";
import { PaneHead } from "../chrome/PaneHead";
import { useLoaded } from "../load";
import { MissionPeek } from "../mission/MissionPeek";
import { peekOf } from "../mission/peek";
import { ErrorCard } from "../ui/ErrorCard";
import { Loading } from "../ui/Loading";
import { Status } from "../ui/Status";
import { headlineStatus } from "../ui/statusOf";
import { humanError } from "../ui/strings";
import { Time } from "../ui/Time";
import { useWorkspace } from "../workspace/store";

type MissionPaneProps = {
  // Its tab's address, which is its own page's.
  to: string;
  slug: string;
};

// MissionPane is a mission, peeked at beside the page: read as its own
// page reads it, and drawn as a MissionPeek. It's read again whenever the
// workspace says the mission's changed (its list's stamp and headline,
// which the window follows already), so a peek left open keeps up
// without a stream of its own.
export function MissionPane({ to, slug }: MissionPaneProps) {
  const listed = useWorkspace((w) => w.missions.find((m) => m.slug === slug));
  const stamp = `${listed?.updated ?? ""}:${listed?.headline.kind ?? ""}:${listed?.crew.length ?? 0}`;
  const { value: d, error, reload } = useLoaded<MissionDetail>(() => getMission(slug), [slug], { keep: true });
  const first = useRef(true);
  useEffect(() => {
    // The first is what was just read.
    if (!first.current) void reload();
    first.current = false;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [stamp]);
  const title = d?.title ?? listed?.title ?? slug;
  const headline = d?.headlines[0] ?? listed?.headline;
  return (
    <DockBody label={title} pad>
      <PaneHead
        title={title}
        to={to}
        sub={
          <>
            {headline && <Status s={headlineStatus(headline)} />}
            {listed?.updated && (
              <span>
                Changed <Time iso={listed.updated} as="relative" />
              </span>
            )}
          </>
        }
      />
      {error && !d && <ErrorCard title="Couldn't read the mission" error={humanError(error)} retry={() => void reload()} />}
      {!d && !error && <Loading what="Reading the mission…" />}
      {d && <MissionPeek {...peekOf(d)} />}
    </DockBody>
  );
}
