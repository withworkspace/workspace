import { Link } from "react-router";
import type { MissionDetail } from "../api";
import { type Refs } from "../ui/hover";
import { HoverCard } from "../ui/hover";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/inline";
import { Status, type StatusInfo } from "../ui/status";
import { Time } from "../ui/time";
import { ArtifactState, artifactTitle, CapsuleBadges, CrewBadge, isArtifact } from "./parts";

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
        <HoverCard key={key} trigger={<span className="ref-chip">{ref}</span>}>
          <span className="hovercard-head">
            <Icon name="gavel" size={13} className="dim" />
            <span className="hovercard-title">Decision {x.id}</span>
            {x.state !== "current" && <span className="chip tone-quiet">{x.state === "superseded" ? "Replaced" : "Withdrawn"}</span>}
          </span>
          <span className="hovercard-strong">
            <Inline text={x.what} />
          </span>
          {x.why && (
            <span className="dim">
              <Inline text={x.why} lines={4} />
            </span>
          )}
          <span className="hovercard-line dim">
            {x.by === "you" ? "You" : x.by}, <Time iso={x.at} as="relative" />
          </span>
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
        <HoverCard key={key} trigger={<span className="ref-chip">{ref}</span>}>
          <span className="hovercard-head">
            <Icon name="message" size={13} className="dim" />
            <span className="hovercard-title">{t.title || t.opening}</span>
          </span>
          <Status s={st} />
          {t.summary && (
            <span className="hovercard-said">
              <Inline text={t.summary} lines={4} />
            </span>
          )}
          <Link to={`/missions/${d.slug}/conversations/${t.id}`} className="hovercard-link">
            Open the conversation <Icon name="arrow-right" size={11} />
          </Link>
        </HoverCard>
      );
    }
    const c = capsules.get(ref);
    if (c)
      return (
        <HoverCard key={key} trigger={<span className="ref-text mono">{ref}</span>}>
          <span className="hovercard-head">
            <Icon name="box" size={13} className="dim" />
            <span className="hovercard-title mono">{ref}</span>
          </span>
          {c.branch && (
            <span className="hovercard-line dim">
              <Icon name="branch" size={11} /> {c.branch}
            </span>
          )}
          <CapsuleBadges c={c} />
          {c.exists && (
            <span className="hovercard-links">
              <Link to={`/missions/${d.slug}/review/${c.repo}/${c.name}`} className="hovercard-link">
                Review its changes <Icon name="arrow-right" size={11} />
              </Link>
              <Link to={`/missions/${d.slug}/files/${c.repo}/${c.name}`} className="hovercard-link">
                Its files <Icon name="arrow-right" size={11} />
              </Link>
            </span>
          )}
        </HoverCard>
      );
    const a = docs.get(ref);
    if (a) {
      const { title, desc } = artifactTitle(a);
      return (
        <HoverCard key={key} trigger={<span className="ref-text">{ref}</span>}>
          <span className="hovercard-head">
            <Icon name={isArtifact(a) ? "artifact" : "file"} size={13} className="dim" />
            <span className="hovercard-title">{title}</span>
          </span>
          <span className="hovercard-line">
            <span className="dim">{a.kind}</span>
            {isArtifact(a) && !!a.versions && <span className="dim">, {a.versions === 1 ? "1 version" : `${a.versions} versions`}</span>} <ArtifactState state={a.state} />
          </span>
          {desc && (
            <span className="dim">
              <Inline text={desc} lines={3} />
            </span>
          )}
          {isArtifact(a) ? (
            <Link to={`/artifacts/${a.ref}`} className="hovercard-link">
              Open the artifact <Icon name="arrow-right" size={11} />
            </Link>
          ) : (
            a.readable && (
              <Link to={`/missions/${d.slug}/artifact?${new URLSearchParams({ ref: a.ref })}`} className="hovercard-link">
                Read it <Icon name="arrow-right" size={11} />
              </Link>
            )
          )}
        </HoverCard>
      );
    }
    return null;
  };
  return { pattern, render };
}
