import type { Artifact } from "../api";
import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";

// MissionOutputThreads is how many threads are open on one of a mission's
// outputs, in the attention tone while some wait on you.
export function MissionOutputThreads({ t }: { t: NonNullable<Artifact["threads"]> }) {
  return (
    <Chip tone={t.waiting ? "attention" : "quiet"} title={`${t.open} open threads${t.waiting ? `, ${t.waiting} waiting on you` : ""}`}>
      <Icon name="message" size={11} />
      {t.open}
      {t.waiting > 0 && ` · ${t.waiting} your turn`}
    </Chip>
  );
}
