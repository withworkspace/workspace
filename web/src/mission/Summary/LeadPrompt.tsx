import { type CrewDetail, getTranscript, type TranscriptEntry } from "../../api";
import { useLoaded } from "../../load";
import { MoreLink } from "../../ui/MoreLink";
import { PromptCard, pendingCall } from "../AgentView";
import { MissionLeadPrompt } from "./MissionLeadPrompt";

// LeadPrompt is what crew are stopped on, answered from the mission's
// page: the step it wants permission for (read from its transcript), or
// its question.
export function LeadPrompt({ slug, crew }: { slug: string; crew: CrewDetail }) {
  const { value: pending } = useLoaded<TranscriptEntry | undefined>(
    crew.activity === "permission" ? () => getTranscript(slug, crew.id).then(pendingCall) : null,
    [slug, crew.id, crew.activity],
    { keep: true },
  );
  return (
    <MissionLeadPrompt>
      <PromptCard slug={slug} crew={crew} pending={pending} compact />
      <MoreLink to={`/missions/${slug}/crew/${crew.id}`}>
        Open {crew.id}'s conversation
      </MoreLink>
    </MissionLeadPrompt>
  );
}
