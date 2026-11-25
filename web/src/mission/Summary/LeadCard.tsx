import type { ReactNode } from "react";
import type { Headline, MissionCard, MissionDetail } from "../../api";
import type { Reload } from "../../load";
import { Clamp } from "../../ui/Clamp";
import { Inline } from "../../ui/Inline";
import { headlineStatus } from "../../ui/statusOf";
import { cardTone } from "../cardTone";
import { CheckingNow } from "../Checking/CheckingNow";
import { checkingTitle } from "../Checking/words";
import { MissionEndingHandedOn } from "../Ending/MissionEndingHandedOn";
import { ZChecks } from "../Ending/ZChecks";
import { LeadActs } from "./LeadActs";
import { MissionLead } from "./MissionLead";
import { MissionLeadNotice } from "./MissionLeadNotice";
import { MissionLeadRequest } from "./MissionLeadRequest";
import { actsShape, actsWide } from "./words";

// LeadCard draws a card as it's given: its title, its sentence, and what
// it offers. The first leads the page (a MissionLead, with the step it's
// on and what crew last said); the rest are notices under it. Nothing
// about how is worked out here.
export function LeadCard({ d, c, lead, reload, step, latest }: { d: MissionDetail; c: MissionCard; lead: boolean; reload: Reload; step?: ReactNode; latest?: ReactNode }) {
  const st = headlineStatus({ kind: c.kind, court: c.court, label: "" } as Headline);
  const title = c.checking ? checkingTitle(c.checking as unknown as NonNullable<Headline["checking"]>) : c.title;
  const detail = c.checking ? undefined : c.detail;
  const long = !!detail && detail.length > 220;
  const offers = !!c.primary || !!c.more?.length;
  const actions = offers ? <LeadActs d={d} c={c} lead={lead} reload={reload} /> : undefined;
  const body = (
    <>
      {c.checking && <CheckingNow c={c.checking as unknown as NonNullable<Headline["checking"]>} />}
      {detail &&
        (c.kind === "resume" || (long && !lead) ? (
          <details className="fold">
            <summary>{c.kind === "resume" ? `What ${c.primary?.target ?? "crew"} will be told` : "Read it all"}</summary>
            <Inline text={detail} />
          </details>
        ) : long ? (
          <Clamp lines={3} more="More" less="Less" of={detail}>
            <Inline text={detail} />
          </Clamp>
        ) : (
          <Inline text={detail} />
        ))}
      {c.nothing && <div className="dim">Nothing for you to do</div>}
      {lead && (c.kind === "to-brief" || c.kind === "briefing") && d.request && <MissionLeadRequest>Requested: {d.request}</MissionLeadRequest>}
      {c.kind === "wrap-up" && d.conclude?.ending && (
        <>
          <ZChecks c={d.conclude} />
          <MissionEndingHandedOn to={d.conclude.to} />
        </>
      )}
      {c.kind === "ended" && d.ending && (
        <>
          <ZChecks c={d.ending} />
          <MissionEndingHandedOn to={d.ending.to} />
        </>
      )}
    </>
  );
  return lead ? (
    <MissionLead tone={cardTone[c.tone]} icon={st.icon} live={st.live} title={title} step={step} actions={actions} wide={actsWide(actsShape(d, c, lead))} latest={latest}>
      {body}
    </MissionLead>
  ) : (
    <MissionLeadNotice icon={st.icon} title={title} actions={actions}>
      {body}
    </MissionLeadNotice>
  );
}
