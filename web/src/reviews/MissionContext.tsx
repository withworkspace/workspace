import { useEffect, useState } from "react";
import { type Capsule, getMission, type Mission } from "../api";
import { Markdown } from "../markdown/LazyMarkdown";
import { OverviewMission } from "./OverviewMission";

// MissionContext is a capsule's overview: the mission it's for, where that
// stands, and its brief, which says what the capsule's work is for.
export function MissionContext({ mission, capsule }: { mission: Mission; capsule: Capsule }) {
  const [brief, setBrief] = useState<string | null>();
  useEffect(() => {
    let stopped = false;
    getMission(mission.slug).then(
      (d) => !stopped && setBrief(d.brief),
      () => !stopped && setBrief(null),
    );
    return () => {
      stopped = true;
    };
  }, [mission.slug]);
  const h = mission.headline;
  return (
    <>
      <h2>Mission</h2>
      <OverviewMission slug={mission.slug} title={mission.title} repo={capsule.repo} name={capsule.name} branch={capsule.branch} />
      {h.text && <p className="dim">{h.text}</p>}
      <h2>Brief</h2>
      {brief === undefined ? (
        <p className="dim">Reading the brief…</p>
      ) : brief === null ? (
        <p className="dim">The mission's brief couldn't be read.</p>
      ) : brief.trim() ? (
        <div className="prose">
          <Markdown text={brief} />
        </div>
      ) : (
        <p className="dim">The mission has no brief yet.</p>
      )}
    </>
  );
}
