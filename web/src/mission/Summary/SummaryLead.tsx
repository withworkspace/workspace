import type { MissionDetail } from "../../api";
import type { Reload } from "../../load";
import { LeadCard } from "./LeadCard";
import { MissionLeadLatest, MissionLeadSaid } from "./MissionLeadLatest";
import { MissionSummarySaid } from "./MissionSummarySaid";
import { stepLine } from "./stepLine";
import { saidLately } from "./words";

// SummaryLead is where the mission stands, as its Summary leads with it:
// its cards as ws says they're drawn (mission.Cards), the first as the
// page's subject, with the step it's on and what crew last said, and
// anything else under it as a notice. One card, while things are as they
// should be. The stories draw it alone (Banners.stories.tsx).
export function SummaryLead({ d, reload }: { d: MissionDetail; reload: Reload }) {
  const said = saidLately(d);
  const step = stepLine(d);
  const latest = said.length > 0 && (
    <MissionLeadLatest>
      {said.map((c) => (
        <MissionLeadSaid key={c.id} who={<>From {c.id}</>}>
          <MissionSummarySaid text={c.lastReply!} />
        </MissionLeadSaid>
      ))}
    </MissionLeadLatest>
  );
  return (
    <>
      {d.cards.map((c, i) => (
        <LeadCard key={`${c.kind}-${c.by ?? ""}-${i}`} d={d} c={c} lead={i === 0} reload={reload} step={i === 0 ? step : undefined} latest={i === 0 ? latest : undefined} />
      ))}
    </>
  );
}
