import type { MissionDetail } from "../api";
import { type Refs } from "../refs/context";
import { HoverCard } from "../ui/HoverCard";
import { Inline } from "../ui/inline";
import { Status, type StatusInfo } from "../ui/Status";
import { Time } from "../ui/Time";
import { ArtifactState, artifactTitle, CapsuleBadges, CrewBadge, isArtifact } from "./parts";
import { MissionCapsuleCard } from "./MissionCapsuleCard";
import { decisionsPath, whoDecided } from "./decisions";
import { MissionDecisionCard } from "./MissionDecisionCard";
import { MissionDocCard } from "./MissionDocCard";
import { MissionRefChip } from "./MissionRefChip";
import { MissionTalkCard } from "./MissionTalkCard";

// missionRefs are the names a mission's page knows, for text that
// mentions them to show each with its hover card: crew (c1), decisions
// (d2), its conversations (t3), its capsules (repo/name) and documents (by
// their path).

const escape = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

export function missionRefs(d: MissionDetail): Refs | undefined {
  const crew = new Map(d.crew.map((c) => [c.id, c]));
  const decisions = new Map(d.decisions.map((x) => [x.id, x]));
  const talks = new Map(d.talks.map((t) => [t.id, t]));
  const capsules = new Map(d.capsules.map((c) => [`${c.repo}/${c.name}`, c]));
  // A document by its ref (records/x:path, mission:path) or its path.
  const docs = new Map<string, (typeof d.artifacts)[number]>();
  for (const a of d.artifacts) {
    docs.set(a.ref, a);
    const path = a.ref.includes(":") ? a.ref.slice(a.ref.indexOf(":") + 1) : "";
    if (path && path.includes("/")) docs.set(path, a);
  }
  const names = [...crew.keys(), ...decisions.keys(), ...talks.keys(), ...capsules.keys(), ...docs.keys()];
  if (!names.length) return undefined;
  names.sort((a, b) => b.length - a.length);
  const pattern = new RegExp(`(?<![\\w/.:-])(?:${names.map(escape).join("|")})(?![\\w/-]|\\.\\w)`, "g");

  const render = (ref: string, key: number) => {
    if (crew.has(ref)) return <CrewBadge key={key} id={ref} />;
    const x = decisions.get(ref);
    if (x)
      return (
        <HoverCard key={key} trigger={<MissionRefChip>{ref}</MissionRefChip>}>
          <MissionDecisionCard
            id={x.id}
            stands={x.state === "current" ? undefined : x.state === "superseded" ? "Replaced" : "Withdrawn"}
            what={<Inline text={x.what} />}
            why={x.why && <Inline text={x.why} lines={4} />}
            by={whoDecided(x.by)}
            at={<Time iso={x.at} as="relative" />}
            to={decisionsPath(d.slug, x.id)}
          />
        </HoverCard>
      );
    const t = talks.get(ref);
    if (t) {
      const st: StatusInfo =
        t.status === "resolved"
          ? { icon: "check-circle", tone: "quiet", words: "Resolved" }
          : t.answering
            ? { icon: "spinner", tone: "progress", words: "Answering", live: true }
            : t.waiting
              ? { icon: "reply", tone: "attention", words: "Your turn" }
              : { icon: "message", tone: "quiet", words: "Open" };
      return (
        <HoverCard key={key} trigger={<MissionRefChip>{ref}</MissionRefChip>}>
          <MissionTalkCard title={t.title || t.opening} status={<Status s={st} />} summary={t.summary && <Inline text={t.summary} lines={4} />} to={`/missions/${d.slug}/conversations/${t.id}`} />
        </HoverCard>
      );
    }
    const c = capsules.get(ref);
    if (c)
      return (
        <HoverCard
          key={key}
          trigger={
            <MissionRefChip words mono>
              {ref}
            </MissionRefChip>
          }
        >
          <MissionCapsuleCard
            name={ref}
            branch={c.branch}
            badges={<CapsuleBadges c={c} />}
            review={c.exists ? `/missions/${d.slug}/review/${c.repo}/${c.name}` : undefined}
            files={c.exists ? `/missions/${d.slug}/files/${c.repo}/${c.name}` : undefined}
          />
        </HoverCard>
      );
    const a = docs.get(ref);
    if (a) {
      const { title, desc } = artifactTitle(a);
      const made = isArtifact(a);
      return (
        <HoverCard key={key} trigger={<MissionRefChip words>{ref}</MissionRefChip>}>
          <MissionDocCard
            artifact={made}
            title={title}
            kind={a.kind}
            versions={made ? a.versions : undefined}
            state={<ArtifactState state={a.state} />}
            desc={desc && <Inline text={desc} lines={3} />}
            to={made ? `/artifacts/${a.ref}` : a.readable ? `/missions/${d.slug}/artifact?${new URLSearchParams({ ref: a.ref })}` : undefined}
          />
        </HoverCard>
      );
    }
    return null;
  };
  return { pattern, render };
}
