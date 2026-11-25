import type { Transcript } from "../api";
import { useTranscript } from "../mission/Crew/useTranscript";
import { stepDoing } from "../ui/commandLabel";
import { useCommandLabels } from "../workspace/store";
import type { CommandLabel } from "../wire.gen";
import { FootCrew } from "./FootCrew";

// How many of a session's latest entries the foot holds: enough that a
// change told over the socket lands in them, and no more.
const held = 12;

// crewDoing is what crew is doing this moment, from its session: its
// latest step, as a run going now shows it ("Editing Foot.tsx"), or
// thinking when what's latest is something said. Nothing until it's read.
export function crewDoing(t: Transcript | undefined, labels: readonly CommandLabel[]): string | undefined {
  if (!t?.available) return undefined;
  const last = t.entries[t.entries.length - 1];
  if (!last) return undefined;
  return last.by === "call" && last.tool ? stepDoing({ ...last, tool: last.tool, input: last.input ?? "" }, labels) : "Thinking";
}

// FootCrewLive is a crew at work in the foot, following its session for
// what it's doing.
export function FootCrewLive({ slug, id, mission }: { slug: string; id: string; mission: string }) {
  const { t } = useTranscript(slug, id, held);
  const labels = useCommandLabels();
  return <FootCrew id={id} mission={mission} doing={crewDoing(t, labels)} to={`/missions/${slug}/crew/${id}`} />;
}
